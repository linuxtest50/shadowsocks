package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type SmartDNSServer struct {
	Address      string
	Port         int
	IPSet        *HashIPSet
	LocalDNS     string
	RemoteDNS    string
	RemoteDNSTcp string
	Conn         *net.UDPConn
	Selector     *DNSResultSelector
	ReadTimeout  time.Duration
}

type DNSResult struct {
	Size   int
	Buffer []byte
	Error  error
}

func (s *SmartDNSServer) Run() {
	s.Selector = NewDNSResultSelector(s.LocalDNS, s.RemoteDNS, s.IPSet)
	udpAddr := fmt.Sprintf("%s:%d", s.Address, s.Port)
	uaddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		log.Printf("Error: cannot resolve UDP address: %s\n", udpAddr)
		os.Exit(1)
	}
	s.Conn, err = net.ListenUDP("udp", uaddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Starting Smart DNS Server at %s\n", uaddr)
	for {
		buf := make([]byte, 4096)
		n, src, err := s.Conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Read packet from UDP error: %v\n", err)
			continue
		}
		go s.HandleUDPPacket(n, src, buf[0:n])
	}
}

func (s *SmartDNSServer) SendPacketTo(target string, buf []byte, reschan chan *DNSResult) {
	var dnstarget = target
	result := DNSResult{Size: 0, Buffer: nil, Error: nil}
	if !strings.Contains(target, ":") {
		dnstarget += ":53"
	}
	remoteAddr, err := net.ResolveUDPAddr("udp", dnstarget)
	if err != nil {
		result.Error = err
		reschan <- &result
		return
	}
	remote, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		result.Error = err
		reschan <- &result
		return
	}
	defer remote.Close()
	_, err = remote.Write(buf)
	if err != nil {
		result.Error = err
		reschan <- &result
		return
	}
	retBuf := make([]byte, 4096)
	remote.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	rn, _, err := remote.ReadFromUDP(retBuf)
	if err != nil {
		result.Error = err
		reschan <- &result
		return
	}
	result.Size = rn
	result.Buffer = retBuf[0:rn]
	result.Error = err
	reschan <- &result
	return
}

func (s *SmartDNSServer) QueryDNSViaTCP(target string, buf []byte) *DNSResult {
	var dnstarget = target
	result := DNSResult{Size: 0, Buffer: nil, Error: nil}
	if !strings.Contains(target, ":") {
		dnstarget += ":53"
	}
	_, err := net.ResolveTCPAddr("tcp", dnstarget)
	if err != nil {
		result.Error = err
		return &result
	}
	msg := new(dns.Msg)
	err = msg.Unpack(buf)
	if err != nil {
		result.Error = err
		return &result
	}
	c := new(dns.Client)
	c.Net = "tcp"
	c.ReadTimeout = s.ReadTimeout
	c.DialTimeout = s.ReadTimeout
	res, _, err := c.Exchange(msg, dnstarget)
	if err != nil {
		result.Error = err
		return &result
	}
	result.Buffer, result.Error = res.Pack()
	result.Size = len(result.Buffer)
	return &result
}

func (s *SmartDNSServer) QueryLocal(buf []byte, reschan chan *DNSResult) {
	s.SendPacketTo(s.LocalDNS, buf, reschan)
}

func (s *SmartDNSServer) QueryRemote(buf []byte, reschan chan *DNSResult) {
	rchan := make(chan *DNSResult, 1)
	s.SendPacketTo(s.RemoteDNS, buf, rchan)
	var udpRes *DNSResult
	udpRes = <-rchan
	if udpRes.Error == nil {
		reschan <- udpRes
		return
	}
	if s.RemoteDNSTcp == "" {
		reschan <- udpRes
		return
	}
	log.Println("Query Remote DNS Via TCP")
	tcpRes := s.QueryDNSViaTCP(s.RemoteDNSTcp, buf)
	reschan <- tcpRes
	return
}

func (s *SmartDNSServer) HandleUDPPacket(n int, src *net.UDPAddr, buf []byte) {
	// We catch panic on this goroutine to prevent system crash on one query
	// got some error.
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	msg, err := s.Selector.UnpackBuffer(buf)
	if err != nil {
		log.Printf("Not a DNS query message")
		return
	}
	qdetail := s.Selector.GetQueryDetails(msg)
	lrchan := make(chan *DNSResult)
	rrchan := make(chan *DNSResult)
	// Query A or AAAA, use Selector to choose best result
	go s.QueryLocal(buf, lrchan)
	go s.QueryRemote(buf, rrchan)
	var lrres, rrres *DNSResult
	lrres = <-lrchan
	rrres = <-rrchan
	if lrres.Error != nil {
		log.Printf("Got error from Local DNS: %v\n", lrres.Error)
	}
	if rrres.Error != nil {
		log.Printf("Got error from Remote DNS: %v\n", rrres.Error)
	}
	var result []byte
	if lrres.Size > 0 && rrres.Size > 0 && s.Selector.IsQueryA(msg) {
		result = s.Selector.SelectResult(lrres.Buffer, rrres.Buffer, qdetail, src)
	} else {
		if lrres.Size > 0 {
			log.Printf("[LSRE] %v Query %s Select local answer on %s\n", src, qdetail, s.LocalDNS)
			result = lrres.Buffer
		} else if rrres.Size > 0 {
			log.Printf("[LERS] %v Query %s Select remote answer on %s\n", src, qdetail, s.RemoteDNS)
			result = rrres.Buffer
		} else {
			log.Printf("[LERE] %v Query %s Cannot resolve!\n", src, qdetail)
			result = nil
		}
	}
	if result != nil {
		s.Conn.WriteToUDP(result, src)
	}
}
