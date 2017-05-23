package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DNSResultSelector struct {
	LocalDNS         string
	RemoteDNS        string
	IPSet            *HashIPSet
	EnableCNAMECheck bool
}

func NewDNSResultSelector(local string, remote string, ipset *HashIPSet, enableCNAMECheck bool) *DNSResultSelector {
	return &DNSResultSelector{
		LocalDNS:         local,
		RemoteDNS:        remote,
		IPSet:            ipset,
		EnableCNAMECheck: enableCNAMECheck,
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
	lanswer, lhascname := s.GetAnswers(lmsg)
	ranswer, rhascname := s.GetAnswers(rmsg)
	linipset := s.AnyInIPSet(lanswer)
	rinipset := s.AnyInIPSet(ranswer)

	if s.EnableCNAMECheck {
		// Local or Remote have CNAME, just use local
		if lhascname && rhascname {
			log.Printf("[LCNRCN] %v Query %s Select local answer on %s", src, qdetail, s.LocalDNS)
			return local
		}

		if !lhascname && rhascname {
			log.Printf("[LARCN] %v Query %s Select remote answer on %s", src, qdetail, s.RemoteDNS)
			return remote
		}
	}

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

func (s *DNSResultSelector) GetAnswers(msg *dns.Msg) ([]string, bool) {
	ret := []string{}
	hascname := false
	for _, answer := range msg.Answer {
		if answer.Header().Rrtype == dns.TypeA {
			ipv4 := answer.(*dns.A).A.String()
			ret = append(ret, ipv4)
		} else if answer.Header().Rrtype == dns.TypeCNAME {
			hascname = true
		}
	}
	return ret, hascname
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
