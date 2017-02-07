package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	kcp "github.com/muss/kcp-go"
	"github.com/muss/smux"
)

const (
	DataShard        int  = 12
	ParityShard      int  = 3
	SocketBufferSize int  = 4194304
	NoDelay          int  = 0
	Interval         int  = 30
	Resend           int  = 5
	NoCongestion     int  = 0
	KCPMtu           int  = 1350
	SendWindow       int  = 1024
	RecvWindow       int  = 1024
	AckNoDelay       bool = true
	KeepAlive        int  = 0
)

var smuxConfig *smux.Config
var KCPSessionCache map[string]*smux.Session
var KCPSessionLock sync.RWMutex

func init() {
	smuxConfig = smux.DefaultConfig()
	smuxConfig.MaxReceiveBuffer = SocketBufferSize
	smuxConfig.KeepAliveInterval = 2 * time.Second
	smuxConfig.KeepAliveTimeout = 4 * time.Second
	KCPSessionCache = make(map[string]*smux.Session)
}

func getKCPAddr(remoteAddr string) (string, error) {
	taddr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return "", err
	}
	host := taddr.IP.String()
	kcpPort := taddr.Port + 10000
	if kcpPort > 32767 {
		return "", errors.New(fmt.Sprintf("KCP Port %d is not valid", kcpPort))
	}
	kcpAddr := fmt.Sprintf("%s:%d", host, kcpPort)
	return kcpAddr, nil
}

func CreateKCPSession(remoteAddr string) (*smux.Session, error) {
	kcpAddr, err := getKCPAddr(remoteAddr)
	if err != nil {
		return nil, err
	}
	block, _ := kcp.NewNoneBlockCrypt([]byte(""))
	kcpConn, err := kcp.DialWithOptions(kcpAddr, block, DataShard, ParityShard)
	if err != nil {
		return nil, err
	}
	err = kcpConn.SetReadBuffer(SocketBufferSize)
	if err != nil {
		kcpConn.Close()
		return nil, err
	}
	err = kcpConn.SetWriteBuffer(SocketBufferSize)
	if err != nil {
		kcpConn.Close()
		return nil, err
	}
	kcpConn.SetStreamMode(true)
	kcpConn.SetNoDelay(NoDelay, Interval, Resend, NoCongestion)
	kcpConn.SetMtu(KCPMtu)
	kcpConn.SetWindowSize(SendWindow, RecvWindow)
	kcpConn.SetACKNoDelay(AckNoDelay)
	kcpConn.SetKeepAlive(KeepAlive)
	sess, err := smux.Client(kcpConn, smuxConfig)
	if err != nil {
		kcpConn.Close()
		return nil, err
	}
	log.Printf("Create KCP Session for %s", kcpAddr)
	return sess, nil
}

func DialKCPConn(remoteAddr string) (net.Conn, error) {
	KCPSessionLock.RLock()
	sess, have := KCPSessionCache[remoteAddr]
	KCPSessionLock.RUnlock()
	if !have || sess.IsClosed() {
		KCPSessionLock.Lock()
		newSess, err := CreateKCPSession(remoteAddr)
		if err != nil {
			KCPSessionLock.Unlock()
			return nil, err
		}
		KCPSessionCache[remoteAddr] = newSess
		KCPSessionLock.Unlock()
		return newSess.OpenStream()
	}
	return sess.OpenStream()
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
