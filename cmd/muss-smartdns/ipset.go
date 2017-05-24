package main

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type HashIPSet struct {
	FileName string
	IPSet    map[string]*net.IPNet
	lock     sync.RWMutex
}

func NewHashIPSet(routeFile string) (*HashIPSet, error) {
	ret := &HashIPSet{
		FileName: routeFile,
		IPSet:    make(map[string]*net.IPNet),
	}
	err := ret.Reload()
	return ret, err
}

func (h *HashIPSet) Reload() error {
	fp, err := os.Open(h.FileName)
	if err != nil {
		return err
	}
	defer fp.Close()
	content, err := ioutil.ReadAll(fp)
	if err != nil {
		return err
	}
	ipset := make(map[string]*net.IPNet)
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
		ipset[ip.String()] = ipnet
	}
	h.lock.Lock()
	h.IPSet = ipset
	h.lock.Unlock()
	log.Println("Load CIDR List Finish")
	return nil
}

func (h *HashIPSet) Contains(ipstr string) bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
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
