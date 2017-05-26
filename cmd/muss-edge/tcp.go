package main

import (
	"fmt"
	"log"
	"net"
)

func runTCPProxy(config *Config) {
	listenAddr := fmt.Sprintf("0.0.0.0:%d", config.ListenTCPPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Start TCP Proxy At:", listenAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept:", err)
			continue
		}
		go handleTCPConnection(conn, config)
	}
}

func connectToBackend(config *Config) (net.Conn, error) {
	backendAddr := config.GetTCPBackendAddr()
	return net.Dial("tcp", backendAddr)
}

func handleTCPConnection(conn net.Conn, config *Config) {
	closed := false
	defer func() {
		if !closed {
			conn.Close()
		}
	}()
	backend, err := connectToBackend(config)
	if err != nil {
		log.Println("Cannot connect to Backend:", err)
		return
	}

	defer func() {
		if !closed {
			backend.Close()
		}
	}()

	go ProxyPipe(conn, backend)
	ProxyPipe(backend, conn)
	closed = true
}
