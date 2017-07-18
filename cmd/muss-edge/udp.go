package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"syscall"
	"time"

	ss "github.com/muss/muss-go/shadowsocks"
)

const UDP_TIMEOUT = 5 * time.Second

type CachedUDPConn struct {
	*net.UDPConn
	srcaddr_index string
}

func NewCachedUDPConn(udpconn *net.UDPConn, index string) *CachedUDPConn {
	return &CachedUDPConn{udpconn, index}
}

func (c *CachedUDPConn) Close() error {
	return c.UDPConn.Close()
}

type NATlist struct {
	sync.Mutex
	conns map[string]*CachedUDPConn
}

var natList = &NATlist{conns: map[string]*CachedUDPConn{}}

func (self *NATlist) Delete(index string) {
	self.Lock()
	c, ok := self.conns[index]
	if ok {
		c.Close()
		delete(self.conns, index)
	}
	defer self.Unlock()
}

func (self *NATlist) Get(index string) (c *CachedUDPConn, ok bool, err error) {
	self.Lock()
	defer self.Unlock()
	c, ok = self.conns[index]
	if !ok {
		//NAT not exists or expired
		//delete(self.conns, index)
		//ok = false
		//full cone
		conn, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IPv6zero,
			Port: 0,
		})
		if err != nil {
			return nil, ok, err
		}
		c = NewCachedUDPConn(conn, index)
		self.conns[index] = c
	}
	err = nil
	return
}

type UDPProxy struct {
	listenAddr string
	backends   []string
	running    bool
	lock       sync.RWMutex
	conn       *net.UDPConn
	timeout    int
}

func NewUDPProxy(listenAddr string, backends []string, timeout int) *UDPProxy {
	return &UDPProxy{
		listenAddr: listenAddr,
		backends:   backends,
		running:    true,
		timeout:    timeout,
	}
}

func (p *UDPProxy) UpdateBackendsAddr(backends []string) error {
	for _, backend := range backends {
		_, err := net.ResolveUDPAddr("udp", backend)
		if err != nil {
			return err
		}
	}
	p.lock.Lock()
	p.backends = backends
	p.lock.Unlock()
	return nil
}

func (p *UDPProxy) UpdateTimeout(timeout int) {
	p.lock.Lock()
	p.timeout = timeout
	p.lock.Unlock()
}

func (p *UDPProxy) GetTimeout() int {
	return p.timeout
}

func (p *UDPProxy) Stop() {
	p.running = false
}

func (p *UDPProxy) getBackendAddr() string {
	return p.backends[0]
}

func (p *UDPProxy) handleUDPPacket(conn *net.UDPConn, n int, src *net.UDPAddr, data []byte) {
	defer HandlePanic()
	defer ss.LeakyBuffer.Put(data)
	p.lock.RLock()
	timeout := time.Duration(p.timeout) * time.Second
	backendAddr := p.getBackendAddr()
	p.lock.RUnlock()
	dst, err := net.ResolveUDPAddr("udp", backendAddr)
	if err != nil {
		log.Printf("Cannot resolve UDP backend address: %v\n", err)
		return
	}

	remote, exist, err := natList.Get(src.String())
	if err != nil {
		return
	}
	if !exist {
		go func() {
			p.pipeloop(conn, src, remote, timeout)
			natList.Delete(src.String())
		}()
	}
	remote.SetDeadline(time.Now().Add(timeout))
	go func() {
		_, err := remote.WriteToUDP(data[0:n], dst)
		if err != nil {
			if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
				log.Println("[udp]write error:", err)
			} else {
				log.Println("[udp]error connecting to:", dst, err)
			}
			natList.Delete(src.String())
		}
	}()
	return
}

func (p *UDPProxy) pipeloop(conn *net.UDPConn, srcaddr *net.UDPAddr, remote *CachedUDPConn, timeout time.Duration) {
	buf := ss.LeakyBuffer.Get()
	defer ss.LeakyBuffer.Put(buf)
	for {
		remote.SetDeadline(time.Now().Add(timeout))
		n, _, err := remote.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
				// log too many open file error
				// EMFILE is process reaches open file limits, ENFILE is system limit
				fmt.Println("[UDP] read error:", err)
			} else if ne.Err.Error() == "use of closed network connection" {
				fmt.Println("[UDP] Connection Closing:", remote.LocalAddr())
			} else {
				fmt.Println("[UDP] error reading from:", remote.LocalAddr(), err)
			}
			return
		}
		go conn.WriteToUDP(buf[0:n], srcaddr)
	}
}

func (p *UDPProxy) Start() error {
	uaddr, err := net.ResolveUDPAddr("udp", p.listenAddr)
	if err != nil {
		log.Println("Error: cannot resolve UDP address: %s\n", p.listenAddr)
		return err
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		log.Println(err)
		return err
	}
	p.conn = conn
	go p.run()
	return nil
}

func (p *UDPProxy) run() {
	log.Println("Start UDP Proxy At:", p.listenAddr, "Backends:", p.backends)
	for {
		if !p.running {
			break
		}
		buf := ss.LeakyBuffer.Get()
		n, src, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Read packet from UDP error: %v\n", err)
			ss.LeakyBuffer.Put(buf)
			continue
		}
		go p.handleUDPPacket(p.conn, n, src, buf)
	}
	log.Println("Stop UDP Proxy At:", p.listenAddr)
}
