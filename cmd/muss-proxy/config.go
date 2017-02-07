package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	ss "github.com/muss/muss-go/shadowsocks"
)

// {
//    "server_password": [
//        ["127.0.0.1:7071", "password", "aes-256-cfb-auth"]
//    ],
//    "user_id": 1234,
//    "auth": true,
//    "timeout": 600,
//    "proxies": [
//        ["127.0.0.1:5353", "114.114.114.114:53", "tcpudp"]
//    ]
// }
type ProxyConfig struct {
	ServerPassword [][]string `json:"server_password"`
	UserID         int        `json:"user_id"`
	LocalAddress   string     `json:"local_address"`
	Proxies        [][]string `json:"proxies"`
	Timeout        int        `json:"timeout"`
	Auth           bool       `json:"auth"`
}

func ParseProxyConfig(path string) (config *ProxyConfig, err error) {
	file, err := os.Open(path) // For read access.
	if err != nil {
		return
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}

	config = &ProxyConfig{}
	if err = json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	timeout := time.Duration(config.Timeout) * time.Second
	ss.SetTimeout(timeout)
	return
}
