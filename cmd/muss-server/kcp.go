package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	kcp "github.com/muss/kcp-go"
	"github.com/muss/smux"
)

const (
	DataShard        int  = 12
	ParityShard      int  = 3
	SocketBufferSize int  = 4194304
	NoDelay          int  = 0
	Interval         int  = 200
	Resend           int  = 3
	NoCongestion     int  = 0
	KCPMtu           int  = 1350
	SendWindow       int  = 1024
	RecvWindow       int  = 1024
	AckNoDelay       bool = true
	KeepAlive        int  = 0
)

func getKCPPort(port string) (int, int, error) {
	localPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, 0, err
	}
	udpPort := localPort + 10000
	if udpPort > 32767 {
		return 0, 0, errors.New(fmt.Sprintf("UDP Port %d is not valid", udpPort))
	}
	return udpPort, localPort, nil
}

func initKCPListener(kcpPort int, localPort int) (*kcp.Listener, error) {
	block, _ := kcp.NewNoneBlockCrypt([]byte(""))
	kcpAddr := fmt.Sprintf("0.0.0.0:%d", kcpPort)
	listener, err := kcp.ListenWithOptions(kcpAddr, block, DataShard, ParityShard)
	if err != nil {
		return nil, err
	}
	var lerr error
	lerr = listener.SetReadBuffer(SocketBufferSize)
	if lerr != nil {
		listener.Close()
		return nil, lerr
	}
	lerr = listener.SetWriteBuffer(SocketBufferSize)
	if lerr != nil {
		listener.Close()
		return nil, lerr
	}
	return listener, nil
}

func initKCPStream(conn *kcp.UDPSession) {
	conn.SetStreamMode(true)
	conn.SetNoDelay(NoDelay, Interval, Resend, NoCongestion)
	conn.SetMtu(KCPMtu)
	conn.SetWindowSize(SendWindow, RecvWindow)
	conn.SetACKNoDelay(AckNoDelay)
	conn.SetKeepAlive(KeepAlive)
}

/*
 * bandwidth unit is Mbps
 */
func UpdateKCPStreamWithBandwidth(conn *kcp.UDPSession, bandwidth int) {
	// wnd = bandwidth * interval / (mtu * 8)
	windowSize := (bandwidth * 1000 * 1000) * Interval / (KCPMtu * 8)
	conn.SetWindowSize(windowSize, windowSize)
}

func handleMux(conn io.ReadWriteCloser, localAddr string) {
	smuxConfig := smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = SocketBufferSize
	smuxConfig.KeepAliveInterval = 2 * time.Second
	smuxConfig.KeepAliveTimeout = 4 * time.Second
	mux, err := smux.Server(conn, smuxConfig)
	if err != nil {
		log.Printf("[KCP] %v", err)
		return
	}
	defer mux.Close()
	for {
		p1, err := mux.AcceptStream()
		if err != nil {
			log.Printf("[KCP] %v", err)
			return
		}
		p2, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
		if err != nil {
			p1.Close()
			log.Println(err)
			continue
		}
		if err := p2.(*net.TCPConn).SetReadBuffer(SocketBufferSize); err != nil {
			log.Println("[KCP] TCP SetReadBuffer:", err)
		}
		if err := p2.(*net.TCPConn).SetWriteBuffer(SocketBufferSize); err != nil {
			log.Println("[KCP] TCP SetWriteBuffer:", err)
		}
		go handleClient(p1, p2)
	}
}

func handleClient(p1, p2 io.ReadWriteCloser) {
	debug.Println("[KCP] stream opened")
	defer debug.Println("[KCP] stream closed")
	defer p1.Close()
	defer p2.Close()

	// start tunnel
	p1die := make(chan struct{})
	go func() { io.Copy(p1, p2); close(p1die) }()

	p2die := make(chan struct{})
	go func() { io.Copy(p2, p1); close(p2die) }()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

func runKCPTunnel(port string) {
	kcpPort, localPort, err := getKCPPort(port)
	if err != nil {
		log.Print(err)
		log.Printf("KCP Tunnel for %d will not started\n", localPort)
		return
	}
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	listener, err := initKCPListener(kcpPort, localPort)
	if err != nil {
		log.Print(err)
		log.Printf("KCP Tunnel for %d will not started\n", localPort)
		return
	}
	log.Printf("KCP Tunnel for %d started at: 0.0.0.0:%d", localPort, kcpPort)

	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			log.Printf("%+v", err)
			continue
		}
		log.Printf("[KCP] Remote Address: %v", conn.RemoteAddr())
		initKCPStream(conn)
		go handleMux(conn, localAddr)
	}
}

func reportKCPStatus() {
	for {
		time.Sleep(2 * time.Second)
		log.Printf("[KCP] Status:\n")
		status := kcp.DefaultSnmp
		keys := status.Header()
		values := status.ToSlice()
		for i := 0; i < len(keys); i++ {
			log.Printf("[KCP]     %s: %s\n", keys[i], values[i])
		}
		log.Printf("[KCP] Status Finish\n")
	}
}
