package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	ss "github.com/muss/muss-go/shadowsocks"
)

var debug ss.DebugLog

func init() {
	rand.Seed(time.Now().Unix())
}

type ServerCipher struct {
	server string
	cipher *ss.Cipher
}

var servers struct {
	srvCipher []*ServerCipher
	failCnt   []int // failed connection count
}

func getRedirAddr(conn net.Conn) ([]byte, string, error) {
	c, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, "", errors.New("only work with TCP connection")
	}
	f, err := c.File()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	fd := f.Fd()
	if err := syscall.SetNonblock(int(fd), true); err != nil {
		return nil, "", err
	}
	return getDestAddrIPv4(fd)
}

func handleConnection(conn net.Conn, userID int, useKCP bool) {
	if debug {
		debug.Printf("socks connect from %s\n", conn.RemoteAddr().String())
	}
	closed := false
	defer func() {
		if !closed {
			conn.Close()
		}
	}()
	rawaddr, addr, err := getRedirAddr(conn)
	if err != nil {
		log.Println("error getting redir addr:", err)
		return
	}
	remote, err := createServerConnWithUserID(rawaddr, addr, userID, useKCP)
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
	debug.Println("closed connection to", addr)
}

func connectToServerWithUserID(serverId int, rawaddr []byte, addr string, userID int, useKCP bool) (remote *ss.Conn, err error) {
	se := servers.srvCipher[serverId]
	if useKCP {
		kcpConn, kerr := DialKCPConn(se.server)
		if kerr != nil {
			err = kerr
		} else {
			remote, err = ss.InitConnWithRawAddrAndUserID(rawaddr, kcpConn, se.cipher.Copy(), ss.UserID2Byte(userID))
		}
	} else {
		remote, err = ss.DialWithRawAddrAndUserID(rawaddr, se.server, se.cipher.Copy(), ss.UserID2Byte(userID))
	}
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

func connectToServer(serverId int, rawaddr []byte, addr string) (remote *ss.Conn, err error) {
	se := servers.srvCipher[serverId]
	remote, err = ss.DialWithRawAddr(rawaddr, se.server, se.cipher.Copy())
	if err != nil {
		log.Println("error connecting to shadowsocks server:", err)
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

// Connection to the server in the order specified in the config. On
// connection failure, try the next server. A failed server will be tried with
// some probability according to its fail count, so we can discover recovered
// servers.
func createServerConnWithUserID(rawaddr []byte, addr string, userID int, useKCP bool) (remote *ss.Conn, err error) {
	const baseFailCnt = 20
	n := len(servers.srvCipher)
	skipped := make([]int, 0)
	for i := 0; i < n; i++ {
		// skip failed server, but try it with some probability
		if servers.failCnt[i] > 0 && rand.Intn(servers.failCnt[i]+baseFailCnt) != 0 {
			skipped = append(skipped, i)
			continue
		}
		remote, err = connectToServerWithUserID(i, rawaddr, addr, userID, useKCP)
		if err == nil {
			return
		}
	}
	// last resort, try skipped servers, not likely to succeed
	for _, i := range skipped {
		remote, err = connectToServerWithUserID(i, rawaddr, addr, userID, useKCP)
		if err == nil {
			return
		}
	}
	return nil, err
}

func runTCP(listenAddr string, userID int, useKCP bool) {
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
		go handleConnection(conn, userID, useKCP)
	}
}

func runNameServer(listenAddr string, userID int) {
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
		go handleUDPPacket(conn, n, src, buf, userID)
	}
}

func enoughOptions(config *ss.Config) bool {
	return config.Server != nil && config.ServerPort != 0 &&
		config.LocalPort != 0 && config.Password != ""
}

func parseServerConfig(config *ss.Config) {
	hasPort := func(s string) bool {
		_, port, err := net.SplitHostPort(s)
		if err != nil {
			return false
		}
		return port != ""
	}

	if len(config.ServerPassword) == 0 {
		method := config.Method
		if config.Auth {
			method += "-auth"
		}
		// only one encryption table
		cipher, err := ss.NewCipher(method, config.Password)
		if err != nil {
			log.Fatal("Failed generating ciphers:", err)
		}
		srvPort := strconv.Itoa(config.ServerPort)
		srvArr := config.GetServerArray()
		n := len(srvArr)
		servers.srvCipher = make([]*ServerCipher, n)

		for i, s := range srvArr {
			if hasPort(s) {
				log.Println("ignore server_port option for server", s)
				servers.srvCipher[i] = &ServerCipher{s, cipher}
			} else {
				servers.srvCipher[i] = &ServerCipher{net.JoinHostPort(s, srvPort), cipher}
			}
		}
	} else {
		// multiple servers
		n := len(config.ServerPassword)
		servers.srvCipher = make([]*ServerCipher, n)

		cipherCache := make(map[string]*ss.Cipher)
		i := 0
		for _, serverInfo := range config.ServerPassword {
			if len(serverInfo) < 2 || len(serverInfo) > 3 {
				log.Fatalf("server %v syntax error\n", serverInfo)
			}
			server := serverInfo[0]
			passwd := serverInfo[1]
			encmethod := ""
			if len(serverInfo) == 3 {
				encmethod = serverInfo[2]
			}
			if !hasPort(server) {
				log.Fatalf("no port for server %s\n", server)
			}
			// Using "|" as delimiter is safe here, since no encryption
			// method contains it in the name.
			cacheKey := encmethod + "|" + passwd
			cipher, ok := cipherCache[cacheKey]
			if !ok {
				var err error
				cipher, err = ss.NewCipher(encmethod, passwd)
				if err != nil {
					log.Fatal("Failed generating ciphers:", err)
				}
				cipherCache[cacheKey] = cipher
			}
			servers.srvCipher[i] = &ServerCipher{server, cipher}
			i++
		}
	}
	servers.failCnt = make([]int, len(servers.srvCipher))
	for _, se := range servers.srvCipher {
		log.Println("available remote server", se.server)
	}
	return
}

func main() {
	log.SetOutput(os.Stdout)

	var configFile, cmdServer, cmdLocal, udpLocal string
	var cmdConfig ss.Config
	var printVer bool
	var useKCP bool

	flag.BoolVar(&printVer, "version", false, "print version")
	flag.StringVar(&configFile, "c", "config.json", "specify config file")
	flag.BoolVar((*bool)(&debug), "d", false, "print debug message")
	flag.BoolVar(&useKCP, "K", false, "use KCP for TCP connection")
	flag.StringVar(&cmdLocal, "l", "127.0.0.1", "Listen address default is 127.0.0.1")
	flag.StringVar(&udpLocal, "L", "127.0.0.1", "UDP listen address default is 127.0.0.1")

	flag.Parse()

	if printVer {
		ss.PrintVersion()
		os.Exit(0)
	}

	cmdConfig.Server = cmdServer
	ss.SetDebug(debug)
	exists, err := ss.IsFileExists(configFile)
	// If no config file in current directory, try search it in the binary directory
	// Note there's no portable way to detect the binary directory.
	binDir := path.Dir(os.Args[0])
	if (!exists || err != nil) && binDir != "" && binDir != "." {
		oldConfig := configFile
		configFile = path.Join(binDir, "config.json")
		log.Printf("%s not found, try config file %s\n", oldConfig, configFile)
	}

	config, err := ss.ParseConfig(configFile)
	if err != nil {
		config = &cmdConfig
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", configFile, err)
			os.Exit(1)
		}
	} else {
		ss.UpdateConfig(config, &cmdConfig)
	}
	if config.Method == "" {
		config.Method = "aes-256-cfb"
	}
	if len(config.ServerPassword) == 0 {
		if !enoughOptions(config) {
			fmt.Fprintln(os.Stderr, "must specify server address, password and both server/local port")
			os.Exit(1)
		}
	} else {
		if config.Password != "" || config.ServerPort != 0 || config.GetServerArray() != nil {
			fmt.Fprintln(os.Stderr, "given server_password, ignore server, server_port and password option:", config)
		}
		if config.LocalPort == 0 {
			fmt.Fprintln(os.Stderr, "must specify local port")
			os.Exit(1)
		}
	}

	parseServerConfig(config)
	if config.EnableDNSProxy {
		TargetNameServer = config.TargetDNSServer
		dnsProxyPort := config.DNSProxyPort
		go runNameServer(fmt.Sprintf("%s:%d", udpLocal, dnsProxyPort), config.UserID)
	}
	//if debug {
	//	go reportKCPStatus()
	//}
	runTCP(cmdLocal+":"+strconv.Itoa(config.LocalPort), config.UserID, useKCP)
}
