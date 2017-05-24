package main

import (
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

type ResolveRule struct {
	FileName   string
	LocalRule  []string
	RemoteRule []string
	lock       sync.RWMutex
}

const (
	RESOLVE_NORMAL int = 0
	RESOLVE_LOCAL  int = 1
	RESOLVE_REMOTE int = 2
)

func (r *ResolveRule) GetResolvType(odomain string) int {
	r.lock.RLock()
	defer r.lock.RUnlock()
	domain := odomain[0 : len(odomain)-1]
	for _, rsuffix := range r.RemoteRule {
		if strings.HasSuffix(domain, rsuffix) {
			return RESOLVE_REMOTE
		}
	}
	for _, lsuffix := range r.LocalRule {
		if strings.HasSuffix(domain, lsuffix) {
			return RESOLVE_LOCAL
		}
	}
	return RESOLVE_NORMAL
}

func NewResolveRule(fname string) (*ResolveRule, error) {
	ret := &ResolveRule{
		FileName:   fname,
		LocalRule:  []string{},
		RemoteRule: []string{},
	}
	err := ret.Reload()
	return ret, err
}

func (r *ResolveRule) Reload() error {
	file, err := os.Open(r.FileName)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	r.parseResolveRuleFile(string(data))
	return nil
}

func (r *ResolveRule) parseResolveRuleFile(data string) {
	localRule := []string{}
	lrmap := make(map[string]int)
	remoteRule := []string{}
	rrmap := make(map[string]int)
	for _, oline := range strings.Split(data, "\n") {
		line := strings.TrimSpace(oline)
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "local" {
			lrmap[value] = 1
		} else if key == "remote" {
			rrmap[value] = 1
		}
	}
	for k, _ := range rrmap {
		remoteRule = append(remoteRule, k)
	}
	for k, _ := range lrmap {
		localRule = append(localRule, k)
	}
	r.lock.Lock()
	r.LocalRule = localRule
	r.RemoteRule = remoteRule
	r.lock.Unlock()
}
