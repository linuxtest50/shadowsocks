package main

import (
	"log"
	"net"
	"sync"
)

type TCPProxy struct {
	listenAddr     string
	backends       []string
	running        bool
	conn           *net.TCPListener
	lock           sync.RWMutex
	timeout        int
	backendChecker *BackendChecker
}

func NewTCPProxy(listenAddr string, backends []string, timeout int) *TCPProxy {
	return &TCPProxy{
		listenAddr:     listenAddr,
		backends:       backends,
		running:        true,
		timeout:        timeout,
		backendChecker: NewBackendChecker(backends),
	}
}

func (p *TCPProxy) Start() error {
	p.backendChecker.Start()
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
	log.Println("Start TCP Proxy At:", p.listenAddr, "Backends:", p.backends)
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
	baddr := p.backendChecker.BestBackend
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

func (p *TCPProxy) UpdateBackendsAddr(backends []string) error {
	for _, backend := range backends {
		_, err := net.ResolveTCPAddr("tcp", backend)
		if err != nil {
			return err
		}
	}
	p.lock.Lock()
	p.backends = backends
	if p.backendChecker != nil {
		p.backendChecker.Stop()
	}
	p.backendChecker = NewBackendChecker(backends)
	p.backendChecker.Start()
	p.lock.Unlock()
	return nil
}

func (p *TCPProxy) Stop() {
	p.running = false
	p.conn.Close()
}

func (p *TCPProxy) UpdateTimeout(timeout int) {
	p.lock.Lock()
	p.timeout = timeout
	p.lock.Unlock()
}

func (p *TCPProxy) GetTimeout() int {
	return p.timeout
}
