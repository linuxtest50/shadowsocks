package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

const GETSOCKOPT = syscall.SYS_GETSOCKOPT

// SOCKS address types as defined in RFC 1928 section 5.
const (
	AtypIPv4       = 1
	AtypDomainName = 3
	AtypIPv6       = 4
)

const (
	SO_ORIGINAL_DST      = 80 // from linux/include/uapi/linux/netfilter_ipv4.h
	IP6T_SO_ORIGINAL_DST = 80 // from linux/include/uapi/linux/netfilter_ipv6/ip6_tables.h
)

func socketcall(call, a0, a1, a2, a3, a4, a5 uintptr) error {
	if _, _, errno := syscall.Syscall6(call, a0, a1, a2, a3, a4, a5); errno != 0 {
		return errno
	}
	return nil
}

func getDestAddrIPv4(fd uintptr) ([]byte, string, error) {
	raw := syscall.RawSockaddrInet4{}
	size := unsafe.Sizeof(raw)
	if err := socketcall(GETSOCKOPT, fd, syscall.IPPROTO_IP, SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&size)), 0); err != nil {
		return nil, "", err
	}
	addr := make([]byte, 1+net.IPv4len+2)
	addr[0] = AtypIPv4
	copy(addr[1:1+net.IPv4len], raw.Addr[:])
	port := (*[2]byte)(unsafe.Pointer(&raw.Port)) // big-endian
	addr[1+net.IPv4len], addr[1+net.IPv4len+1] = port[0], port[1]
	lenAddr := len(addr)
	iport := binary.BigEndian.Uint16(addr[lenAddr-2 : lenAddr])
	host := net.IP(addr[1 : 1+net.IPv4len]).String()
	host = net.JoinHostPort(host, strconv.Itoa(int(iport)))
	return addr, host, nil
}

func getDestAddrIPv6(fd uintptr) ([]byte, string, error) {
	raw := syscall.RawSockaddrInet6{}
	size := unsafe.Sizeof(raw)
	if err := socketcall(GETSOCKOPT, fd, syscall.IPPROTO_IPV6, IP6T_SO_ORIGINAL_DST, uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&size)), 0); err != nil {
		return nil, "", err
	}
	addr := make([]byte, 1+net.IPv6len+2)
	addr[0] = AtypIPv6
	copy(addr[1:1+net.IPv6len], raw.Addr[:])
	port := (*[2]byte)(unsafe.Pointer(&raw.Port)) // big-endian
	addr[1+net.IPv6len], addr[1+net.IPv6len+1] = port[0], port[1]
	host := net.IP(addr[1 : 1+net.IPv6len]).String()
	lenAddr := len(addr)
	iport := binary.BigEndian.Uint16(addr[lenAddr-2 : lenAddr])
	host = net.JoinHostPort(host, strconv.Itoa(int(iport)))
	return addr, host, nil
}
