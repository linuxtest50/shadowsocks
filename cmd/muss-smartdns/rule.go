package main

import (
	"io/ioutil"
	"os"
	"strings"
)

type ResolveRule struct {
	LocalRule  []string
	RemoteRule []string
}

const (
	RESOLVE_NORMAL int = 0
	RESOLVE_LOCAL  int = 1
	RESOLVE_REMOTE int = 2
)

func (r *ResolveRule) GetResolvType(odomain string) int {
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
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return parseResolveRuleFile(string(data)), nil
}

func parseResolveRuleFile(data string) *ResolveRule {
	ret := ResolveRule{
		LocalRule:  []string{},
		RemoteRule: []string{},
	}
	for _, oline := range strings.Split(data, "\n") {
		line := strings.TrimSpace(oline)
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "local" {
			ret.LocalRule = append(ret.LocalRule, value)
		} else if key == "remote" {
			ret.RemoteRule = append(ret.RemoteRule, value)
		}
	}
	return &ret
}
