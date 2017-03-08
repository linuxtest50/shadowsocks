# muss-redir 说明文档

muss-redir 为支持 iptables redirect 功能的代理程序，可以通过设置 iptables 的 redirect 代理所有流量到 muss。

## iptables 配置

filter 表配置

```
iptables -A INPUT -i eth0 -p tcp --dport 7070 -j DROP
```

nat 表配置

```
iptables -t nat -N MUSS
iptables -t nat -A MUSS -d [MUSS-SERVER_IP] -j RETURN
iptables -t nat -A MUSS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A MUSS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A MUSS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A MUSS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A MUSS -d 192.168.0.0/16 -j RETURN
iptables -t nat -A MUSS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A MUSS -d 240.0.0.0/4 -j RETURN
iptables -t nat -A MUSS -p tcp -j REDIRECT --to-ports 7070

# 重定向本机的 TCP 请求到 muss-redir
iptables -t nat -A OUTPUT -p tcp -j MUSS

# 重定向其他机器的 TCP 请求到 muss-redir，改规则用于网关服务，请自行修改 10.0.0.0/8 为内网 IP 段
iptables -t nat -A PREROUTING -s 10.0.0.0/8 -p tcp -j MUSS

# 网关模式
iptables -t nat -I POSTROUTING -o eth0 -j MASQUERADE
```

## sysctl.conf 配置

```
net.ipv4.ip_forward = 1
```

修改之后执行

```
# sysctl -p
```

## muss-redir 配置: /etc/muss/config.json

```
{
    "auth": true,
    "local_port": 7070,
    "server_password": [
        [
            "[MUSS-SERVER_IP]",
            "[PASSWORD]",
            "aes-128-cfb-auth"
        ]
    ],
    "user_id": [USER_ID],
    "enable_dns_proxy": true,
    "target_dns_server": "8.8.8.8:53",
    "dns_proxy_port": 53
}
```

## /etc/resolv.conf 配置

```
nameserver 127.0.0.1
nameserver 114.114.114.114
nameserver 8.8.8.8
```

## muss-redir 启动命令行

```
# ./muss-redir -c /etc/muss/config.json -l 0.0.0.0 -L 0.0.0.0
```

## 设想可用组网架构

```
  10.0.0.2                   10.0.0.1       x.x.x.x           GFW
 +--------+                 +-----------------------+          +         y.y.y.y
 | Client | <-- Private --> |     Gateway Server    |          +         +-------------+             +--------+
 +--------+     Network     | muss-redir + iptables | <-- muss + TCP --> | muss-server | <-- TCP --> | Server |
 gw 10.0.0.1                +-----------------------+          +         +-------------+             +--------+
                                                               +
```
