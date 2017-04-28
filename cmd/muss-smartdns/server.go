package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type SmartDNSServer struct {
	Address     string
	Port        int
	IPSet       *HashIPSet
	LocalDNS    string
	RemoteDNS   string
	Conn        *net.UDPConn
	Selector    *DNSResultSelector
	ReadTimeout time.Duration
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

func (s *SmartDNSServer) SendPacketTo(target string, buf []byte) (int, []byte, error) {
	var dnstarget = target
	if !strings.Contains(target, ":") {
		dnstarget += ":53"
	}
	remoteAddr, err := net.ResolveUDPAddr("udp", dnstarget)
	if err != nil {
		return 0, nil, err
	}
	remote, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		return 0, nil, err
	}
	defer remote.Close()
	_, err = remote.Write(buf)
	if err != nil {
		return 0, nil, err
	}
	remote.SetReadDeadline(time.Now().Add(s.ReadTimeout))
	retBuf := make([]byte, 4096)
	rn, _, err := remote.ReadFromUDP(retBuf)
	return rn, retBuf[0:rn], err
}

func (s *SmartDNSServer) HandleUDPPacket(n int, src *net.UDPAddr, buf []byte) {
	msg, err := s.Selector.UnpackBuffer(buf)
	if err != nil {
		log.Printf("Not a DNS query message")
		return
	}
	qdetail := s.Selector.GetQueryDetails(msg)
	// Query A or AAAA, use Selector to choose best result
	nl, localResult, err := s.SendPacketTo(s.LocalDNS, buf)
	if err != nil {
		log.Printf("Got error from Local DNS: %v\n", err)
	}
	nr, remoteResult, err := s.SendPacketTo(s.RemoteDNS, buf)
	if err != nil {
		log.Printf("Got error from Remote DNS: %v\n", err)
	}
	var result []byte
	if nl > 0 && nr > 0 && s.Selector.IsQueryA(msg) {
		result = s.ChooseResult(localResult, remoteResult, qdetail)
	} else {
		if nl > 0 {
			log.Printf("[LSRE] Query %s Select local answer on %s\n", qdetail, s.LocalDNS)
			result = localResult
		} else if nr > 0 {
			log.Printf("[LERS] Query %s Select remote answer on %s\n", qdetail, s.RemoteDNS)
			result = remoteResult
		} else {
			log.Printf("[LERE] Query %s Cannot resolve!\n", qdetail)
			result = nil
		}
	}
	if result != nil {
		s.Conn.WriteToUDP(result, src)
	}
}

func (s *SmartDNSServer) ChooseResult(localResult []byte, remoteResult []byte, qdetail string) []byte {
	return s.Selector.SelectResult(localResult, remoteResult, qdetail)
}
