package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const VERSION = "1.0.0"

func WaitSignal(config *Config) {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGHUP)
	for sig := range sigChan {
		if sig == syscall.SIGHUP {
			// Reload resolve rule file
			err := config.Reload()
			if err != nil {
				log.Println("Reload Config Error:", err)
			} else {
				log.Println("Reload Config Success")
			}
		} else {
			log.Fatal("Server Exit\n")
		}
	}
}

func main() {
	log.SetOutput(os.Stdout)

	var showVersion bool
	var configFile string

	flag.BoolVar(&showVersion, "v", false, "Print version")
	flag.StringVar(&configFile, "c", "", "Config file")
	flag.Parse()

	if showVersion {
		fmt.Printf("muss-edge version %s\n", VERSION)
		os.Exit(0)
	}

	if configFile == "" {
		fmt.Printf("Require config file\n")
		os.Exit(1)
	}
	config, err := NewConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	go runTCPProxy(config)
	go runUDPProxy(config)
	WaitSignal(config)
}
