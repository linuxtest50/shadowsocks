package main

import (
	"log"
	"math/rand"
	"net"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

func runTCPProxy(listenAddr string, remoteAddr string, userID int) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("starting local socks5 server at %v ...\n", listenAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept:", err)
			continue
		}
		go handleTCPConnection(conn, userID, remoteAddr)
	}
}

func handleTCPConnection(conn net.Conn, userID int, remoteAddr string) {
	if debug {
		debug.Printf("socks connect from %s\n", conn.RemoteAddr().String())
	}
	closed := false
	defer func() {
		if !closed {
			conn.Close()
		}
	}()
	remote, err := createServerConnWithUserID(remoteAddr, userID)
	if err != nil {
		if len(servers.srvCipher) > 1 {
			log.Println("Failed connect to all avaiable shadowsocks server")
		}
		return
	}
	defer func() {
		if !closed {
			remote.Close()
		}
	}()

	go ss.PipeThenClose(conn, remote)
	ss.PipeThenClose(remote, conn)
	closed = true
	debug.Println("closed connection to", remoteAddr)
}

func createServerConnWithUserID(remoteAddr string, userID int) (remote *ss.Conn, err error) {
	const baseFailCnt = 20
	n := len(servers.srvCipher)
	skipped := make([]int, 0)
	for i := 0; i < n; i++ {
		// skip failed server, but try it with some probability
		if servers.failCnt[i] > 0 && rand.Intn(servers.failCnt[i]+baseFailCnt) != 0 {
			skipped = append(skipped, i)
			continue
		}
		remote, err = connectToServerWithUserID(i, remoteAddr, userID)
		if err == nil {
			return
		}
	}
	// last resort, try skipped servers, not likely to succeed
	for _, i := range skipped {
		remote, err = connectToServerWithUserID(i, remoteAddr, userID)
		if err == nil {
			return
		}
	}
	return nil, err
}

func generateRawAddress(addr string) []byte {
	const (
		typeIPv4 = 1 // type is ipv4 address
		typeDm   = 3 // type is domain address
		typeIPv6 = 4 // type is ipv6 address

		lenIPv4 = 1 + net.IPv4len + 2 // 1addrType + ipv4 + 2port
		lenIPv6 = 1 + net.IPv6len + 2 // 1addrType + ipv6 + 2port
	)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)
	addrLen := len(tcpAddr.IP)
	buf := make([]byte, addrLen+3)
	if addrLen == net.IPv4len {
		buf[0] = typeIPv4
	} else if addrLen == net.IPv6len {
		buf[0] = typeIPv6
	}
	copy(buf[1:], tcpAddr.IP)
	copy(buf[1+addrLen:], ss.Port2Byte(tcpAddr.Port))
	return buf
}

func connectToServerWithUserID(serverId int, addr string, userID int) (remote *ss.Conn, err error) {
	rawaddr := generateRawAddress(addr)
	se := servers.srvCipher[serverId]
	remote, err = ss.DialWithRawAddrAndUserID(rawaddr, se.server, se.cipher.Copy(), ss.UserID2Byte(userID))
	if err != nil {
		log.Println("error connecting to shadowsocks server:[TCP]", err)
		const maxFailCnt = 30
		if servers.failCnt[serverId] < maxFailCnt {
			servers.failCnt[serverId]++
		}
		return nil, err
	}
	debug.Printf("connected to %s via %s\n", addr, se.server)
	servers.failCnt[serverId] = 0
	return
}
