package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DNSResultSelector struct {
	LocalDNS  string
	RemoteDNS string
	IPSet     *HashIPSet
}

func NewDNSResultSelector(local string, remote string, ipset *HashIPSet) *DNSResultSelector {
	return &DNSResultSelector{
		LocalDNS:  local,
		RemoteDNS: remote,
		IPSet:     ipset,
	}
}

func (s *DNSResultSelector) SelectResult(local []byte, remote []byte, qdetail string, src *net.UDPAddr) []byte {
	lmsg := new(dns.Msg)
	lerr := lmsg.Unpack(local)
	rmsg := new(dns.Msg)
	rerr := rmsg.Unpack(remote)
	// If we got all error just return local
	if lerr != nil && rerr != nil {
		return local
	}
	if rerr != nil {
		return local
	}
	if lerr != nil {
		return remote
	}
	lanswer := s.GetAnswers(lmsg)
	ranswer := s.GetAnswers(rmsg)
	linipset := s.AnyInIPSet(lanswer)
	rinipset := s.AnyInIPSet(ranswer)

	// Local return China IP, remote return not China IP, use remote
	if linipset && !rinipset {
		log.Printf("[LCRR] %v Query %s Select remote answer on %s", src, qdetail, s.RemoteDNS)
		return remote
	}
	// Local return not China IP, remote return not China IP, use remote
	if !linipset && !rinipset {
		log.Printf("[LRRR] %v Query %s Select remote answer on %s", src, qdetail, s.RemoteDNS)
		return remote
	}
	// Local return China IP, remote return China IP, use local
	if linipset && rinipset {
		log.Printf("[LCRC] %v Query %s Select local answer on %s", src, qdetail, s.LocalDNS)
		return local
	}
	// Local return not ChinaIP, remote return China IP, use local
	// I don't think we can touch this condition.
	if !linipset && rinipset {
		log.Printf("[LRRC] %v Query %s Select local answer on %s", src, qdetail, s.LocalDNS)
		return local
	}
	// if not match above just return local
	// actually we should not reach this code
	return local
}

func (s *DNSResultSelector) AnyInIPSet(answers []string) bool {
	for _, answer := range answers {
		if s.IPSet.Contains(answer) {
			return true
		}
	}
	return false
}

func (s *DNSResultSelector) GetAnswers(msg *dns.Msg) []string {
	ret := []string{}
	for _, answer := range msg.Answer {
		if answer.Header().Rrtype == dns.TypeA {
			ipv4 := answer.(*dns.A).A.String()
			ret = append(ret, ipv4)
		}
	}
	return ret
}

func (s *DNSResultSelector) UnpackBuffer(query []byte) (*dns.Msg, error) {
	msg := new(dns.Msg)
	err := msg.Unpack(query)
	return msg, err
}

func (s *DNSResultSelector) GetQueryDetails(msg *dns.Msg) string {
	var ret = []string{}
	for _, q := range msg.Question {
		ret = append(ret, fmt.Sprintf("%s %s", q.Name, dns.TypeToString[q.Qtype]))
	}
	return strings.Join(ret, ", ")
}

func (s *DNSResultSelector) IsQueryA(msg *dns.Msg) bool {
	for _, q := range msg.Question {
		if q.Qtype == dns.TypeA {
			return true
		}
	}
	return false
}
