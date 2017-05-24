package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const VERSION = "1.1.0"

func WaitSignal(resolveRule *ResolveRule, ipset *HashIPSet) {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGHUP)
	for sig := range sigChan {
		if sig == syscall.SIGHUP {
			// Reload resolve rule file
			if resolveRule != nil {
				log.Println("Reload resolve rule")
				resolveRule.Reload()
			} else {
				log.Println("[WARN] No resolve rule need reload")
			}
			ipset.Reload()
			log.Println("Reload ipset")
		} else {
			log.Fatal("Server Exit\n")
		}
	}
}

func main() {
	log.SetOutput(os.Stdout)

	var showVersion, enableCNAMECheck bool
	var bindAddr, routeFile, localDNS, remoteDNS, remoteDNSTcp, resolveRuleFile string
	var port, timeout int

	flag.BoolVar(&showVersion, "v", false, "Print version")
	flag.StringVar(&bindAddr, "b", "0.0.0.0", "Bind address")
	flag.IntVar(&port, "p", 53, "Listen port")
	flag.StringVar(&routeFile, "c", "", "Path to China route file")
	flag.StringVar(&localDNS, "l", "114.114.114.114", "DNS in China")
	flag.StringVar(&remoteDNS, "r", "8.8.8.8", "DNS out of China")
	flag.StringVar(&remoteDNSTcp, "R", "8.8.8.8", "DNS out of China via TCP")
	flag.IntVar(&timeout, "t", 1000, "Read timeout in ms")
	flag.BoolVar(&enableCNAMECheck, "C", false, "Enable CNAME check")
	flag.StringVar(&resolveRuleFile, "f", "", "Resolve rule file")
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

	resolveRule, err := NewResolveRule(resolveRuleFile)
	if err != nil {
		log.Println("[WARN] Cannot load resolve rule:", err)
	}

	server := &SmartDNSServer{
		Address:          bindAddr,
		Port:             port,
		IPSet:            ipset,
		LocalDNS:         localDNS,
		RemoteDNS:        remoteDNS,
		RemoteDNSTcp:     remoteDNSTcp,
		ReadTimeout:      time.Duration(timeout) * time.Millisecond,
		EnableCNAMECheck: enableCNAMECheck,
		ResolveRule:      resolveRule,
	}
	go server.Run()
	WaitSignal(resolveRule, ipset)
}
