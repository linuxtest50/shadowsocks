package main

import (
	"io"
	"log"
)

type MussProxy interface {
	Start() error
	Stop()
	GetBackendAddr() string
	UpdateBackendAddr(string) error
	GetTimeout() int
	UpdateTimeout(int)
}

func HandlePanic() {
	if err := recover(); err != nil {
		log.Println(err)
	}
}

func ProxyPipe(p1, p2 io.ReadWriteCloser) {
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
