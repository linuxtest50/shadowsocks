package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

const VERSION = "1.0.0"

func main() {
	log.SetOutput(os.Stdout)

	var showVersion bool
	var bindAddr, routeFile, localDNS, remoteDNS string
	var port, timeout int

	flag.BoolVar(&showVersion, "v", false, "Print version")
	flag.StringVar(&bindAddr, "b", "0.0.0.0", "Bind address")
	flag.IntVar(&port, "p", 53, "Listen port")
	flag.StringVar(&routeFile, "c", "", "Path to China route file")
	flag.StringVar(&localDNS, "l", "114.114.114.114", "DNS in China")
	flag.StringVar(&remoteDNS, "r", "8.8.8.8", "DNS out of China")
	flag.IntVar(&timeout, "t", 500, "Read timeout in ms")
	flag.Parse()

	if showVersion {
		fmt.Printf("muss-smartdns version %s\n", VERSION)
		os.Exit(0)
	}

	ipset, err := NewHashIPSet(routeFile)
	if err != nil {
		fmt.Printf("Cannot Load China route file: %v\n", err)
		os.Exit(1)
	}

	server := &SmartDNSServer{
		Address:     bindAddr,
		Port:        port,
		IPSet:       ipset,
		LocalDNS:    localDNS,
		RemoteDNS:   remoteDNS,
		ReadTimeout: time.Duration(timeout) * time.Millisecond,
	}
	server.Run()
}
