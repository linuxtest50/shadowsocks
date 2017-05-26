package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

type Config struct {
	// Private fields
	fileName string
	rwLock   sync.RWMutex

	// Public fields
	BackendTCPServer string `json:"backend_tcp_server"`
	BackendUDPServer string `json:"backend_udp_server"`
	ListenTCPPort    int    `json:"listen_tcp_port"`
	ListenUDPPort    int    `json:"listen_udp_port"`
	UDPTimeout       int    `json:"udp_timeout"`
}

func NewConfig(fileName string) (*Config, error) {
	ret := &Config{
		fileName: fileName,
	}
	err := ret.LoadConfig()
	return ret, err
}

func (c *Config) parseConfig() (*Config, error) {
	file, err := os.Open(c.fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	ncfg := &Config{}
	if err = json.Unmarshal(data, ncfg); err != nil {
		return nil, err
	}

	return ncfg, nil
}

func (c *Config) LoadConfig() error {
	ncfg, err := c.parseConfig()
	if err != nil {
		return err
	}
	c.rwLock.Lock()
	c.BackendTCPServer = ncfg.BackendTCPServer
	c.BackendUDPServer = ncfg.BackendUDPServer
	c.ListenTCPPort = ncfg.ListenTCPPort
	c.ListenUDPPort = ncfg.ListenUDPPort
	c.SetTimeout(ncfg.UDPTimeout)
	c.rwLock.Unlock()
	return nil
}

func (c *Config) SetTimeout(udpTimeout int) {
	if udpTimeout == 0 {
		c.UDPTimeout = 5
	} else {
		c.UDPTimeout = udpTimeout
	}
}

func (c *Config) Reload() error {
	ncfg, err := c.parseConfig()
	if err != nil {
		return err
	}
	c.rwLock.Lock()
	c.BackendTCPServer = ncfg.BackendTCPServer
	c.BackendUDPServer = ncfg.BackendUDPServer
	c.SetTimeout(ncfg.UDPTimeout)
	c.rwLock.Unlock()
	return nil
}

func (c *Config) GetTCPBackendAddr() string {
	c.rwLock.RLock()
	defer c.rwLock.RUnlock()
	return c.BackendTCPServer
}

func (c *Config) GetUDPBackendAddr() string {
	c.rwLock.RLock()
	defer c.rwLock.RUnlock()
	return c.BackendUDPServer
}
