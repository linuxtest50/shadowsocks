package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const VERSION = "1.0.0"

type ProxyContainer struct {
	Config  *Config
	Proxies map[string]MussProxy
}

func NewProxyContainer(config *Config) *ProxyContainer {
	return &ProxyContainer{
		Config:  config,
		Proxies: make(map[string]MussProxy),
	}
}

func (c *ProxyContainer) Start() {
	for _, pcfg := range c.Config.Proxies {
		key := fmt.Sprintf("%s %s", pcfg.Protocol, pcfg.Frontend)
		_, have := c.Proxies[key]
		if have {
			log.Println("Already have frontend", key, "ignore it")
			continue
		}
		err := c.startNewProxy(key, pcfg)
		if err != nil {
			log.Println("Start new proxy", key, "error:", err)
		}
	}
}

func (c *ProxyContainer) startNewProxy(key string, pcfg *Proxy) error {
	if pcfg.Protocol == "tcp" {
		proxy := NewTCPProxy(pcfg.Frontend, pcfg.Backends, pcfg.Timeout)
		err := proxy.Start()
		if err != nil {
			return err
		}
		c.Proxies[key] = proxy
	} else if pcfg.Protocol == "udp" {
		proxy := NewUDPProxy(pcfg.Frontend, pcfg.Backends, pcfg.Timeout)
		proxy.UpdateTimeout(pcfg.Timeout)
		err := proxy.Start()
		if err != nil {
			return err
		}
		c.Proxies[key] = proxy
	}
	return nil
}

func (c *ProxyContainer) Reload() {
	c.Config.Reload()
	for _, pcfg := range c.Config.Proxies {
		key := fmt.Sprintf("%s %s", pcfg.Protocol, pcfg.Frontend)
		proxy, have := c.Proxies[key]
		if have {
			// Change Configuration
			if pcfg.Timeout != proxy.GetTimeout() {
				proxy.UpdateTimeout(pcfg.Timeout)
				log.Println("Update proxy", key, "timeout")
			}
			err := proxy.UpdateBackendsAddr(pcfg.Backends)
			if err != nil {
				log.Println("Proxy", key, "update backend error:", err)
			} else {
				log.Println("Update proxy", key, "backend")
			}
		} else {
			// Add new proxy
			err := c.startNewProxy(key, pcfg)
			if err != nil {
				log.Println("Start new proxy", key, "error:", err)
			}
		}
	}
}

func WaitSignal(container *ProxyContainer) {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGHUP)
	for sig := range sigChan {
		if sig == syscall.SIGHUP {
			// Reload resolve rule file
			log.Println("Reloading")
			container.Reload()
			log.Println("Reload Finish")
		} else {
			log.Fatal("Server Exit\n")
		}
	}
}

func main() {
	log.SetOutput(os.Stdout)

	var showVersion bool
	var configFile string

	flag.BoolVar(&showVersion, "v", false, "Print version")
	flag.StringVar(&configFile, "c", "", "Config file")
	flag.Parse()

	if showVersion {
		fmt.Printf("muss-edge version %s\n", VERSION)
		os.Exit(0)
	}

	if configFile == "" {
		fmt.Printf("Require config file\n")
		os.Exit(1)
	}
	config, err := NewConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	container := NewProxyContainer(config)
	container.Start()
	WaitSignal(container)
}
