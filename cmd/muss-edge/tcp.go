package main

import (
	"log"
	"net"
	"sync"
)

type TCPProxy struct {
	listenAddr  string
	backendAddr string
	running     bool
	conn        *net.TCPListener
	lock        sync.RWMutex
	timeout     int
}

func NewTCPProxy(listenAddr string, backendAddr string, timeout int) *TCPProxy {
	return &TCPProxy{
		listenAddr:  listenAddr,
		backendAddr: backendAddr,
		running:     true,
		timeout:     timeout,
	}
}

func (p *TCPProxy) Start() error {
	taddr, err := net.ResolveTCPAddr("tcp", p.listenAddr)
	if err != nil {
		log.Println("Error: cannot resolve tcp address: %s\n", p.listenAddr)
		return err
	}
	ln, err := net.ListenTCP("tcp", taddr)
	if err != nil {
		log.Println(err)
		return err
	}
	p.conn = ln
	go p.run()
	return nil
}

func (p *TCPProxy) run() {
	log.Println("Start TCP Proxy At:", p.listenAddr, "Backend:", p.backendAddr)
	for {
		if !p.running {
			break
		}
		conn, err := p.conn.Accept()
		if err != nil {
			log.Println("accept:", err)
			continue
		}
		go p.handleTCPConnection(conn)
	}
	log.Println("Stop TCP Proxy At:", p.listenAddr)
}

func (p *TCPProxy) connectToBackend() (net.Conn, error) {
	p.lock.RLock()
	baddr := p.backendAddr
	p.lock.RUnlock()
	return net.Dial("tcp", baddr)
}

func (p *TCPProxy) handleTCPConnection(conn net.Conn) {
	defer HandlePanic()
	closed := false
	defer func() {
		if !closed {
			conn.Close()
		}
	}()
	backend, err := p.connectToBackend()
	if err != nil {
		log.Println("Cannot connect to Backend:", err)
		return
	}

	defer func() {
		if !closed {
			backend.Close()
		}
	}()

	ProxyPipe(conn, backend)
	closed = true
}

func (p *TCPProxy) UpdateBackendAddr(backendAddr string) error {
	_, err := net.ResolveTCPAddr("tcp", backendAddr)
	if err != nil {
		return err
	}
	p.lock.Lock()
	p.backendAddr = backendAddr
	p.lock.Unlock()
	return nil
}

func (p *TCPProxy) Stop() {
	p.running = false
	p.conn.Close()
}

func (p *TCPProxy) GetBackendAddr() string {
	return p.backendAddr
}

func (p *TCPProxy) UpdateTimeout(timeout int) {
	p.lock.Lock()
	p.timeout = timeout
	p.lock.Unlock()
}

func (p *TCPProxy) GetTimeout() int {
	return p.timeout
}
