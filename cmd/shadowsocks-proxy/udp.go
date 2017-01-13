package main

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"os"

	ss "github.com/shadowsocks/shadowsocks-go/shadowsocks"
)

const (
	typeIPv4 = 1 // type is ipv4 address
	typeIPv6 = 4 // type is ipv6 address
)

func runUDPProxy(listenAddr string, remoteAddr string, userID int) {
	uaddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		log.Printf("Error: cannot resolve UDP address: %v\n", listenAddr)
		os.Exit(1)
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Starting local Name Server at %s\n", uaddr)
	for {
		buf := ss.LeakyBuffer.Get()
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Read packet from UDP error: %v\n", err)
			continue
		}
		go handleUDPPacket(conn, n, src, buf, userID, remoteAddr)
	}
}

func dialUDPConnection(serverId int) (*ss.UDPConn, error) {
	srv := servers.srvCipher[serverId]
	srvAddr, err := net.ResolveUDPAddr("udp", srv.server)
	if err != nil {
		return nil, err
	}
	remote, err := net.DialUDP("udp", nil, srvAddr)
	if err != nil {
		log.Println("error connecting to shadowsocks server:[UDP]", err)
		const maxFailCnt = 30
		if servers.failCnt[serverId] < maxFailCnt {
			servers.failCnt[serverId]++
		}
		return nil, err
	}
	debug.Printf("connected to %s for UDP\n", srv.server)
	servers.failCnt[serverId] = 0
	ssremote := ss.NewUDPConn(remote, srv.cipher.Copy())
	return ssremote, nil
}

func chooseRemoteServer() (*ss.UDPConn, error) {
	const baseFailCnt = 20
	n := len(servers.srvCipher)
	skipped := make([]int, 0)
	var lastErr error
	for i := 0; i < n; i++ {
		if servers.failCnt[i] > 0 && rand.Intn(servers.failCnt[i]+baseFailCnt) != 0 {
			skipped = append(skipped, i)
			continue
		}
		remote, err := dialUDPConnection(i)
		if err == nil {
			return remote, nil
		}
		lastErr = err
	}
	for _, i := range skipped {
		remote, err := dialUDPConnection(i)
		if err == nil {
			return remote, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func GenerateSSUDPData(buf []byte, n int, auth bool, remoteAddr string) ([]byte, int, error) {
	targetAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return nil, 0, err
	}
	addrLen := len(targetAddr.IP)
	dataLen := 1 + addrLen + 2 + n
	data := make([]byte, dataLen)
	if addrLen == net.IPv4len {
		data[0] = typeIPv4
	} else if addrLen == net.IPv6len {
		data[0] = typeIPv6
	} else {
		return nil, 0, errors.New("Unknown address type")
	}
	if auth {
		data[0] = data[0] | ss.OneTimeAuthMask
	}
	copy(data[1:], targetAddr.IP)
	copy(data[1+addrLen:], ss.Port2Byte(targetAddr.Port))
	copy(data[1+addrLen+2:], buf[:n])
	return data, dataLen, nil
}

func ParseSSUDPResponse(buf []byte, n int) []byte {
	addrType := buf[0] & ss.AddrMask
	addrLen := 0
	if addrType == typeIPv6 {
		addrLen = net.IPv6len
	} else if addrType == typeIPv4 {
		addrLen = net.IPv4len
	}
	dataPos := 1 + addrLen + 2
	return buf[dataPos:n]
}

func handleUDPPacket(conn *net.UDPConn, n int, src *net.UDPAddr, buf []byte, userID int, remoteAddr string) {
	remote, err := chooseRemoteServer()
	if err != nil {
		log.Println("Got error when choose shadowsocks server:[UDP]", err)
		return
	}
	defer remote.Close()
	data, _, err := GenerateSSUDPData(buf, n, remote.IsOta(), remoteAddr)
	if err != nil {
		log.Println("Got error when generate data:[UDP]", err)
		return
	}
	remote.WriteWithUserID(data, ss.UserID2Byte(userID))
	retBuf := make([]byte, 4096)
	rn, err := remote.Read(retBuf)
	if err != nil {
		log.Println("Got error when receive data:[UDP]", err)
		return
	}
	retData := ParseSSUDPResponse(retBuf, rn)
	conn.WriteToUDP(retData, src)
}
