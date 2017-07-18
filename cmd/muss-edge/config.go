package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Proxy struct {
	Protocol string   `json:"protocol"`
	Frontend string   `json:"frontend"`
	Backends []string `json:"backends"`
	Timeout  int      `json:"timeout"`
}

type Config struct {
	// Private fields
	fileName string

	// Public fields
	Proxies []*Proxy `json:"proxies"`
}

func NewConfig(fileName string) (*Config, error) {
	ret := &Config{
		fileName: fileName,
	}
	err := ret.Reload()
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

func (c *Config) Reload() error {
	ncfg, err := c.parseConfig()
	if err != nil {
		return err
	}
	c.Proxies = ncfg.Proxies
	c.check()
	return nil
}

func (c *Config) check() {
	for _, pcfg := range c.Proxies {
		if pcfg.Protocol == "udp" {
			if pcfg.Timeout == 0 {
				pcfg.Timeout = 5
			}
		}
	}
}
