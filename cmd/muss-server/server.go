package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"

	ss "github.com/muss/muss-go/shadowsocks"
)

const (
	idType  = 0 // address type index
	idIP0   = 1 // ip addres start index
	idDmLen = 1 // domain address length index
	idDm0   = 2 // domain address start index

	typeIPv4 = 1 // type is ipv4 address
	typeDm   = 3 // type is domain address
	typeIPv6 = 4 // type is ipv6 address

	lenIPv4     = net.IPv4len + 2 // ipv4 + 2port
	lenIPv6     = net.IPv6len + 2 // ipv6 + 2port
	lenDmBase   = 2               // 1addrLen + 2port, plus addrLen
	lenHmacSha1 = 10
)

var debug ss.DebugLog

func getRequest(conn *ss.Conn, auth bool) (host string, ota bool, err error) {
	ss.SetReadTimeout(conn)

	// buf size should at least have the same size with the largest possible
	// request size (when addrType is 3, domain name has at most 256 bytes)
	// 1(addrType) + 1(lenByte) + 256(max length address) + 2(port) + 10(hmac-sha1)
	buf := make([]byte, 274)
	// read till we get possible domain length field
	if _, err = io.ReadFull(conn, buf[:idType+1]); err != nil {
		return
	}

	var reqStart, reqEnd int
	addrType := buf[idType]
	switch addrType & ss.AddrMask {
	case typeIPv4:
		reqStart, reqEnd = idIP0, idIP0+lenIPv4
	case typeIPv6:
		reqStart, reqEnd = idIP0, idIP0+lenIPv6
	case typeDm:
		if _, err = io.ReadFull(conn, buf[idType+1:idDmLen+1]); err != nil {
			return
		}
		reqStart, reqEnd = idDm0, int(idDm0+buf[idDmLen]+lenDmBase)
	default:
		err = fmt.Errorf("addr type %d not supported", addrType&ss.AddrMask)
		return
	}

	if _, err = io.ReadFull(conn, buf[reqStart:reqEnd]); err != nil {
		return
	}

	// Return string for typeIP is not most efficient, but browsers (Chrome,
	// Safari, Firefox) all seems using typeDm exclusively. So this is not a
	// big problem.
	switch addrType & ss.AddrMask {
	case typeIPv4:
		host = net.IP(buf[idIP0 : idIP0+net.IPv4len]).String()
	case typeIPv6:
		host = net.IP(buf[idIP0 : idIP0+net.IPv6len]).String()
	case typeDm:
		host = string(buf[idDm0 : idDm0+buf[idDmLen]])
	}
	if CheckBlackList(host) {
		err = fmt.Errorf("Host %s is in Black List", host)
		return
	}
	// parse port
	port := binary.BigEndian.Uint16(buf[reqEnd-2 : reqEnd])
	host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	// if specified one time auth enabled, we should verify this
	if auth || addrType&ss.OneTimeAuthMask > 0 {
		ota = true
		if _, err = io.ReadFull(conn, buf[reqEnd:reqEnd+lenHmacSha1]); err != nil {
			return
		}
		iv := conn.GetIv()
		key := conn.GetKey()
		actualHmacSha1Buf := ss.HmacSha1(append(iv, key...), buf[:reqEnd])
		if !bytes.Equal(buf[reqEnd:reqEnd+lenHmacSha1], actualHmacSha1Buf) {
			err = fmt.Errorf("verify one time auth failed, iv=%v key=%v data=%v", iv, key, buf[:reqEnd])
			return
		}
	}
	return
}

func handleConnection(conn *ss.Conn, auth bool, userID int) {
	var host string

	conn.UserID = uint32(userID)
	conn.GetUserStatisticService().IncConnections(conn.UserID)

	// function arguments are always evaluated, so surround debug statement
	// with if statement
	if debug {
		debug.Printf("new client %s->%s\n", conn.RemoteAddr().String(), conn.LocalAddr())
	}
	closed := false
	defer func() {
		if debug {
			debug.Printf("closed pipe %s<->%s\n", conn.RemoteAddr(), host)
		}
		if !closed {
			conn.Close()
		}
	}()

	host, ota, err := getRequest(conn, auth)
	if err != nil {
		log.Println("error getting request", conn.RemoteAddr(), conn.LocalAddr(), err)
		return
	}
	debug.Println("connecting", host)
	remote, err := net.Dial("tcp", host)
	if err != nil {
		if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
			// log too many open file error
			// EMFILE is process reaches open file limits, ENFILE is system limit
			log.Println("dial error:", err)
		} else {
			log.Println("error connecting to:", host, err)
		}
		return
	}
	defer func() {
		if !closed {
			remote.Close()
		}
	}()
	if debug {
		debug.Printf("piping %s<->%s ota=%v connOta=%v", conn.RemoteAddr(), host, ota, conn.IsOta())
	}
	if ota {
		go ss.PipeThenCloseOta(conn, remote)
	} else {
		go ss.PipeThenClose(conn, remote)
	}
	ss.PipeThenClose(remote, conn)
	closed = true
	return
}

func waitSignal(enableProfile bool) {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGHUP)
	for sig := range sigChan {
		if sig == syscall.SIGHUP {
		} else {
			if enableProfile {
				pprof.StopCPUProfile()
			}
			log.Fatal("Server Exit\n")
		}
	}
}

func handleAccepted(conn net.Conn, auth bool, cipherCache, writeBucketCache, readBucketCache *LRU) {
	lcfg := GetLicenseLimit()
	if lcfg.IsExpired() {
		debug.Printf("License is Expired!")
		conn.Close()
		return
	}
	var err error
	buf := make([]byte, 4)
	if _, err = io.ReadFull(conn, buf); err != nil {
		log.Printf("Read UserID error\n")
		conn.Close()
		return
	}
	userID := ss.Byte2UserID(buf)
	// log.Printf("Got New Connection for UserID: %d\n", userID)
	password, bandwidth := getPasswordAndBandwidth(userID)
	if password == "" {
		log.Printf("Error do not have user for ID: %d\n", userID)
		conn.Close()
		return
	}
	if bandwidth > lcfg.MaxBandwidth {
		bandwidth = lcfg.MaxBandwidth
	}
	// Creating cipher upon first connection.
	cipher, have := cipherCache.Get(userID)
	us := ss.GetUserStatisticService()
	us.IncInBytes(uint32(userID), 4)
	if !have {
		cipher, err = ss.NewCipher(config.Method, password)
		if err != nil {
			log.Printf("Error generating cipher for UserID: %d %v\n", userID, err)
			conn.Close()
			return
		}
		log.Printf("Create cipher for UserID: %d on TCP", userID)
		cipherCache.Add(userID, cipher)
	}
	pcipher := cipher.(*ss.Cipher)
	ssconn := ss.NewConn(conn, pcipher.Copy())
	ssconn.WriteBucket = getOrCreateBucket(writeBucketCache, userID, bandwidth)
	ssconn.ReadBucket = getOrCreateBucket(readBucketCache, userID, bandwidth)
	handleConnection(ssconn, auth, userID)
}

func runTCPWithUserID(port string, auth bool, writeBucketCache, readBucketCache *LRU) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Printf("error listening TCP port %v: %v\n", port, err)
		os.Exit(1)
	}
	cipherCache, err := NewLRU(10000, nil)
	if err != nil {
		log.Printf("Error: Cannot create cipher cache!")
		os.Exit(1)
	}
	log.Printf("server listening TCP port %v ...\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			// listener maybe closed to update password
			log.Printf("accept error: %v\n", err)
			continue
		}
		go handleAccepted(conn, auth, cipherCache, writeBucketCache, readBucketCache)
	}
}

func handleReadFromUDP(conn *net.UDPConn, auth bool, n int, src *net.UDPAddr, data []byte, cipherCache, writeBucketCache, readBucketCache *LRU) {
	// At here we add panic recover to prevent encrypt bug.
	// If we got panic here, just ignore this UDP package send.
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	defer ss.LeakyBuffer.Put(data)
	lcfg := GetLicenseLimit()
	if lcfg.IsExpired() {
		debug.Printf("License is Expired!")
		return
	}
	var err error
	if n < 4 {
		log.Printf("Read UserID error\n")
		return
	}
	buf := make([]byte, 4)
	copy(buf, data[:4])
	userID := ss.Byte2UserID(buf)
	log.Printf("Got New Connection for UserID: %d\n", userID)
	password, bandwidth := getPasswordAndBandwidth(userID)
	if password == "" {
		log.Printf("Error do not have user for ID: %d\n", userID)
		return
	}
	if bandwidth > lcfg.MaxBandwidth {
		bandwidth = lcfg.MaxBandwidth
	}
	// Creating cipher upon first connection.
	cipher, have := cipherCache.Get(userID)
	us := ss.GetUserStatisticService()
	us.IncInBytes(uint32(userID), n)
	if !have {
		cipher, err = ss.NewCipher(config.Method, password)
		if err != nil {
			log.Printf("Error generating cipher for UserID: %d %v\n", userID, err)
			return
		}
		log.Printf("Create cipher for UserID: %d on UDP\n", userID)
		cipherCache.Add(userID, cipher)
	}
	pcipher := cipher.(*ss.Cipher)
	ddata := ss.LeakyBuffer.Get()
	dn, iv, err := ss.UDPDecryptData(n, data, pcipher, ddata)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	udpConn := ss.NewUDPConn(conn, pcipher)
	udpConn.UserID = uint32(userID)
	udpConn.WriteBucket = getOrCreateBucket(writeBucketCache, userID, bandwidth)
	udpConn.ReadBucket = getOrCreateBucket(readBucketCache, userID, bandwidth)
	go udpConn.HandleUDPConnection(dn, src, ddata, auth, iv)
}

func runUDPWithUserID(port string, auth bool, writeBucketCache, readBucketCache *LRU) {
	port_i, _ := strconv.Atoi(port)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv6zero,
		Port: port_i,
	})
	if err != nil {
		log.Printf("error listening UDP port %v: %v", port, err)
		os.Exit(1)
	}
	cipherCache, err := NewLRU(10000, nil)
	if err != nil {
		log.Printf("Error: Cannot create cipher cache!")
		os.Exit(1)
	}
	log.Printf("server listening UDP port %v ...\n", port)
	for {
		buf := ss.LeakyBuffer.Get()
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Read packet from UDP error: %v\n", err)
			continue
		}
		go handleReadFromUDP(conn, auth, n, src, buf, cipherCache, writeBucketCache, readBucketCache)
	}
}

func getOrCreateBucket(cache *LRU, userID int, bandwidth int) *ss.Bucket {
	if bandwidth <= 0 {
		return nil
	}
	var bucket *ss.Bucket
	cbucket, have := cache.Get(userID)
	rate := bandwidth * 1000 * 1000 / 8
	var bursting int64 = 4096
	if !have {
		// we should create a bucket
		bucket = ss.NewBucketWithRate(float64(rate), bursting, int64(bandwidth))
		cache.Add(userID, bucket)
	} else {
		bucket = cbucket.(*ss.Bucket)
		if bucket.OriginRate != int64(bandwidth) {
			// For now we just update the rate for TokenBucket is OK
			bucket.UpdateRate(float64(rate), int64(bandwidth))
		}
	}
	return bucket
}

func getPasswordAndBandwidth(userID int) (string, int) {
	if config.UseDatabase {
		return getPasswordAndBandwidthFromDatabase(userID)
	} else {
		return getPasswordFromConfig(userID), -1
	}
}

func getPasswordFromConfig(userID int) string {
	uidPwd := config.UserIDPassword
	uidstr := fmt.Sprintf("%d", userID)
	password, have := uidPwd[uidstr]
	if !have {
		return ""
	}
	return password
}

func enoughOptions(config *ss.Config) bool {
	return config.ServerPort != 0 && config.Password != ""
}

func unifyPortPassword(config *ss.Config) (err error) {
	if len(config.PortPassword) == 0 { // this handles both nil PortPassword and empty one
		if !enoughOptions(config) {
			fmt.Fprintln(os.Stderr, "must specify both port and password")
			return errors.New("not enough options")
		}
		port := strconv.Itoa(config.ServerPort)
		config.PortPassword = map[string]string{port: config.Password}
	}
	return
}

var configFile string
var config *ss.Config

func main() {
	log.SetOutput(os.Stdout)

	var cmdConfig ss.Config
	var printVer bool
	var core int
	var profileVer bool
	var blackListFile string
	var enableKcp bool

	flag.BoolVar(&printVer, "version", false, "print version")
	flag.BoolVar(&profileVer, "P", false, "Enable profile, profile result file will stored to ./shadowsocks-server.prof")
	flag.StringVar(&configFile, "c", "config.json", "specify config file")
	flag.IntVar(&core, "core", 0, "maximum number of CPU cores to use, default is determinied by Go runtime")
	flag.BoolVar((*bool)(&debug), "d", false, "print debug message")
	flag.StringVar(&blackListFile, "b", "", "specify black list file")
	flag.BoolVar(&enableKcp, "K", false, "Enable KCP tunnel for muss")

	flag.Parse()

	if printVer {
		ss.PrintVersion()
		os.Exit(0)
	}

	if profileVer {
		pfp, err := os.Create("shadowsocks-server.prof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(pfp)
	}

	ss.SetDebug(debug)

	if strings.HasSuffix(cmdConfig.Method, "-auth") {
		cmdConfig.Method = cmdConfig.Method[:len(cmdConfig.Method)-5]
		cmdConfig.Auth = true
	}

	if blackListFile != "" {
		err := LoadBlackList(blackListFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v, Just ignore black list\n", blackListFile, err)
		}
	}

	var err error
	config, err = ss.ParseConfig(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", configFile, err)
			os.Exit(1)
		}
		config = &cmdConfig
	} else {
		ss.UpdateConfig(config, &cmdConfig)
	}
	if config.Method == "" {
		config.Method = "aes-256-cfb"
	}
	if err = ss.CheckCipherMethod(config.Method); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = unifyPortPassword(config); err != nil {
		os.Exit(1)
	}
	if core > 0 {
		runtime.GOMAXPROCS(core)
	}
	if config.UseDatabase {
		err = initDB(config.DatabaseURL)
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
		err = initLicense()
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		} else {
			lcfg := GetLicenseLimit()
			if lcfg.IsExpired() {
				fmt.Println("License is Expired")
			} else {
				fmt.Println("License is Valid")
			}
			fmt.Printf("Expire: %v, Max Users: %d, Max Servers: %d, Max Bandwidth: %d\n", lcfg.Expire, lcfg.MaxUsers, lcfg.MaxServers, lcfg.MaxBandwidth)
		}
	}
	if config.UseRedis {
		err = initRedis(config.RedisServer)
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
	}
	// Start User Statistic Service
	ss.CreateUserStatisticService()

	writeBucketCache, err := NewLRU(10000, nil)
	if err != nil {
		log.Printf("Error: Cannot create write bucket cache!")
		os.Exit(1)
	}
	readBucketCache, err := NewLRU(10000, nil)
	if err != nil {
		log.Printf("Error: Cannot create read bucket cache!")
		os.Exit(1)
	}
	for port, _ := range config.PortPassword {
		go runTCPWithUserID(port, config.Auth, writeBucketCache, readBucketCache)
		go runUDPWithUserID(port, config.Auth, writeBucketCache, readBucketCache)
		if enableKcp {
			go runKCPTunnel(port)
		}
	}
	if debug {
		go reportKCPStatus()
	}
	go StartStatisticServer("127.0.0.1:8080")

	waitSignal(profileVer)
}
