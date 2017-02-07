package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"

	ss "github.com/muss/muss-go/shadowsocks"
)

var debug ss.DebugLog

type ServerCipher struct {
	server string
	cipher *ss.Cipher
}

var servers struct {
	srvCipher []*ServerCipher
	failCnt   []int // failed connection count
}

func hasPort(s string) bool {
	_, port, err := net.SplitHostPort(s)
	if err != nil {
		return false
	}
	return port != ""
}

func parseServerConfig(config *ProxyConfig) {
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
	servers.failCnt = make([]int, len(servers.srvCipher))
	for _, se := range servers.srvCipher {
		log.Println("available remote server", se.server)
	}
	return
}

type ProxyInfo struct {
	LocalAddr  string
	RemoteAddr string
	EnableTCP  bool
	EnableUDP  bool
}

func parseProxies(config *ProxyConfig) []ProxyInfo {
	n := len(config.Proxies)
	ret := make([]ProxyInfo, n)
	for i, proxyInfo := range config.Proxies {
		if len(proxyInfo) != 3 {
			log.Fatalf("proxy %v syntax error\n", proxyInfo)
		}
		localAddr := proxyInfo[0]
		remoteAddr := proxyInfo[1]
		mode := proxyInfo[2]
		if !hasPort(localAddr) {
			log.Fatalf("no port for local address %s\n", localAddr)
		}
		if !hasPort(remoteAddr) {
			log.Fatalf("no port for remote address %s\n", remoteAddr)
		}
		if mode != "tcp" && mode != "udp" && mode != "tcpudp" {
			log.Fatalf("mode is not correct %s is not in [tcp|udp|tcpudp]", mode)
		}
		enableTCP := false
		enableUDP := false
		if mode == "tcp" || mode == "tcpudp" {
			enableTCP = true
		}
		if mode == "udp" || mode == "tcpudp" {
			enableUDP = true
		}
		ret[i] = ProxyInfo{
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			EnableTCP:  enableTCP,
			EnableUDP:  enableUDP,
		}
	}
	return ret
}

func waitSignal() {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGHUP)
	for sig := range sigChan {
		if sig == syscall.SIGHUP {
		} else {
			log.Fatal("Server Exit\n")
		}
	}
}

func main() {
	log.SetOutput(os.Stdout)
	var configFile string
	var printVer bool
	var useKCP bool

	flag.BoolVar(&printVer, "version", false, "print version")
	flag.StringVar(&configFile, "c", "config.json", "specify config file")
	flag.BoolVar((*bool)(&debug), "d", false, "print debug message")
	flag.BoolVar(&useKCP, "K", false, "use KCP for TCP connections")

	flag.Parse()

	if printVer {
		ss.PrintVersion()
		os.Exit(0)
	}

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

	config, err := ParseProxyConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", configFile, err)
		os.Exit(1)
	}
	if len(config.ServerPassword) == 0 {
		fmt.Fprintln(os.Stderr, "must specify server address, password and both server/local port")
		os.Exit(1)
	}

	parseServerConfig(config)
	proxies := parseProxies(config)
	for _, proxyInfo := range proxies {
		listenAddr := proxyInfo.LocalAddr
		remoteAddr := proxyInfo.RemoteAddr
		if proxyInfo.EnableTCP {
			go runTCPProxy(listenAddr, remoteAddr, config.UserID, useKCP)
			log.Printf("Start TCP Proxy: %s to %s\n", listenAddr, remoteAddr)
		}
		if proxyInfo.EnableUDP {
			go runUDPProxy(listenAddr, remoteAddr, config.UserID)
			log.Printf("Start UDP Proxy: %s to %s\n", listenAddr, remoteAddr)
		}
	}
	waitSignal()
}
