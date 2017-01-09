package main

import (
	"bufio"
	"os"
)

var blackList map[string]bool

func init() {
	blackList = make(map[string]bool)
}

func LoadBlackList(fname string) error {
	fp, err := os.Open(fname)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(fp)
	for {
		line, _, _ := reader.ReadLine()
		if len(line) == 0 {
			break
		}
		blackList[string(line)] = true
	}
	return nil
}

func CheckBlackList(host string) bool {
	_, have := blackList[host]
	return have
}
