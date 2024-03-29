# shadowsocks-go

Current version: 1.1.5 [![Build Status](https://travis-ci.org/shadowsocks/shadowsocks-go.png?branch=master)](https://travis-ci.org/shadowsocks/shadowsocks-go)

shadowsocks-go is a lightweight tunnel proxy which can help you get through firewalls. It is a port of [shadowsocks](https://github.com/clowwindy/shadowsocks).

The protocol is compatible with the origin shadowsocks (if both have been upgraded to the latest version).

**Note `server_password` option syntax changed in 0.6.2, the client now connects to servers in the order specified in the config.**

**Please develop on the latest develop branch if you want to send pull request.**

# Install

Download precompiled binarys from the [release page](https://github.com/shadowsocks/shadowsocks-go/releases). (All compiled with cgo disabled, except the mac version.)

You can also install from source (assume you have go installed):

```
# on server
go get github.com/shadowsocks/shadowsocks-go/cmd/shadowsocks-server
# on client
go get github.com/shadowsocks/shadowsocks-go/cmd/shadowsocks-local
```

It's recommended to disable cgo when compiling shadowsocks-go. This will prevent the go runtime from creating too many threads for dns lookup.

# Usage

Both the server and client program will look for `config.json` in the current directory. You can use `-c` option to specify another configuration file.

Configuration file is in json format and has the same syntax with [shadowsocks-nodejs](https://github.com/clowwindy/shadowsocks-nodejs/). You can download the sample [`config.json`](https://github.com/shadowsocks/shadowsocks-go/blob/master/config.json), change the following values:

```
server          your server ip or hostname
server_port     server port
local_port      local socks5 proxy port
method          encryption method, null by default (table), the following methods are supported:
                    aes-128-cfb, aes-192-cfb, aes-256-cfb, bf-cfb, cast5-cfb, des-cfb, rc4-md5, chacha20, salsa20, rc4, table
password        a password used to encrypt transfer
timeout         server option, in seconds
```

Run `shadowsocks-server` on your server. To run it in the background, run `shadowsocks-server > log &`.

On client, run `shadowsocks-local`. Change proxy settings of your browser to

```
SOCKS5 127.0.0.1:local_port
```

## About encryption methods

AES is recommended for shadowsocks-go. [Intel AES Instruction Set](http://en.wikipedia.org/wiki/AES_instruction_set) will be used if available and can make encryption/decryption very fast. To be more specific, **`aes-128-cfb` is recommended as it is faster and [secure enough](https://www.schneier.com/blog/archives/2009/07/another_new_aes.html)**.

**rc4 and table encryption methods are deprecated because they are not secure.**

### One Time Auth

Append `-auth` to the encryption method to enable [One Time Auth (OTA)](https://shadowsocks.org/en/spec/one-time-auth.html).

- For server: this will **force client use OTA**, non-OTA connection will be dropped. Otherwise, both OTA and non-OTA clients can connect
- For client: the `-A` command line option can also enable OTA

## Command line options

Command line options can override settings from configuration files. Use `-h` option to see all available options.

```
shadowsocks-local -s server_address -p server_port -k password
    -m aes-128-cfb -c config.json
    -b local_address -l local_port
shadowsocks-server -p server_port -k password
    -m aes-128-cfb -c config.json
    -t timeout
```

Use `-d` option to enable debug message.

## Use multiple servers on client

```
server_password    specify multiple server and password, server should be in the form of host:port
```

Here's a sample configuration [`client-multi-server.json`](https://github.com/shadowsocks/shadowsocks-go/blob/master/sample-config/client-multi-server.json). Given `server_password`, client program will ignore `server_port`, `server` and `password` options.

Servers are chosen in the order specified in the config. If a server can't be connected (connection failure), the client will try the next one. (Client will retry failed server with some probability to discover server recovery.)

## Multiple users with different passwords on server

The server can support users with different passwords. Each user will be served by a unique port. Use the following options on the server for such setup:

```
port_password   specify multiple ports and passwords to support multiple users
```

Here's a sample configuration [`server-multi-port.json`](https://github.com/shadowsocks/shadowsocks-go/blob/master/sample-config/server-multi-port.json). Given `port_password`, server program will ignore `server_port` and `password` options.

### Update port password for a running server

Edit the config file used to start the server, then send `SIGHUP` to the server process.

# Note to OpenVZ users

**Use OpenVZ VM that supports vswap**. Otherwise, the OS will incorrectly account much more memory than actually used. shadowsocks-go on OpenVZ VM with vswap takes about 3MB memory after startup. (Refer to [this issue](https://github.com/shadowsocks/shadowsocks-go/issues/3) for more details.)

If vswap is not an option and memory usage is a problem for you, try [shadowsocks-libev](https://github.com/madeye/shadowsocks-libev).

# Design For Multi User In Single Port Mode

In shadowsocks first connection, client will send cipher’s IV to server first, and then do the rest works. If we want to use single port with multi user mode, we can send a integer (4 bytes) first and then cipher’s IV at every connection created.

In server side, we use a map to store userID and it’s cipher. In client side, every connection created to server we should send 4 bytes userID first and then rest data.

## For Multi User MySQL support

Sample Server Side Configuration File:

config.json

```
{
    "port_password": {
        "8387": "Fake"
    },
    "method": "aes-256-cfb-auth",
    "timeout":600,
    "use_database": true,
    "database_url": "root:password@tcp(127.0.0.1:3306)/ssuser?charset=utf8"
}
```

Sample Client Side Configuration File:

config.json

```
{
    "local_port": 7070,
    "method": "aes-256-cfb-auth",
    "auth": true,
    "timeout": 600,
    "user_id": 1000,
    "server_password": [
        ["127.0.0.1:7071", "123456", "aes-256-cfb-auth"]
    ]
}
```

Create table SQL:

```
CREATE TABLE `user` (
  `userid` int(11) NOT NULL DEFAULT '0',
  `password` varchar(255) DEFAULT NULL,
  `status` varchar(20) DEFAULT NULL,
  `bandwidth` int(11) DEFAULT NULL,
  PRIMARY KEY (`userid`),
  UNIQUE KEY `userid` (`userid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8
```

status: Enabled, Disabled

SQLs for some function:

Add User:

```
INSERT INTO user (userid, password, status, bandwidth) VALUES ('1000', 'password', 'Enabled', '10');
```

Disable User:

```
UPDATE user SET status='Disabled' WHERE userid='1000';
```

Enable User:

```
UPDATE user SET status='Enabled' WHERE userid='1000';
```

Change Password:

```
UPDATE user SET password='PASSWORD' WHERE userid='1000';
```

Delete User:

```
DELETE FROM user WHERE userid='1001';
```
