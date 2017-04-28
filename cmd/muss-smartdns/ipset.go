package main

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
)

type HashIPSet struct {
	IPSet map[string]*net.IPNet
}

func NewHashIPSet(routeFile string) (*HashIPSet, error) {
	ret := &HashIPSet{
		IPSet: make(map[string]*net.IPNet),
	}
	err := ret.LoadRouteFile(routeFile)
	return ret, err
}

func (h *HashIPSet) LoadRouteFile(routeFile string) error {
	fp, err := os.Open(routeFile)
	if err != nil {
		return err
	}
	defer fp.Close()
	content, err := ioutil.ReadAll(fp)
	if err != nil {
		return err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		tline := strings.TrimSpace(line)
		if tline == "" {
			continue
		}
		ip, ipnet, err := net.ParseCIDR(tline)
		if err != nil {
			log.Printf("Error on parse CIDR: %v\n", tline)
			continue
		}
		h.IPSet[ip.String()] = ipnet
	}
	log.Println("Load CIDR List Finish")
	return nil
}

func (h *HashIPSet) Contains(ipstr string) bool {
	ip := net.ParseIP(ipstr).To4()
	for i := 31; i > 0; i-- {
		mask := net.CIDRMask(i, 32)
		mip := ip.Mask(mask)
		ipnet, have := h.IPSet[mip.String()]
		if have && ipnet.Contains(ip) {
			return true
		}
	}
	return false
}
