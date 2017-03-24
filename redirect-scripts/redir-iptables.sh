#!/bin/bash
PUB_ETH=eth0
CHNROUTE_CONFIG=/etc/muss/chnroute.txt
CHNROUTE_PATCH=/etc/muss/chnroute.patch

reload_ipset() {
    ipset flush chnroute
    for cidr in `cat ${CHNROUTE_CONFIG}`; do
        ipset add chnroute $cidr
    done
    if [ -f ${CHNROUTE_PATCH} ]; then
        for cidr in `cat ${CHNROUTE_PATCH}`; do
            ipset add chnroute $cidr
        done
    fi
}

setup_ipset() {
    have_set=`ipset list -t | grep "Name: chnroute"`
    if [ -z $have_set ]; then
        ipset create chnroute hash:net maxelem 65536
    fi
    reload_ipset
}

setup_gateway_mode() {
    sysctl net.ipv4.ip_forward=1
    iptables -t nat -I POSTROUTING -o ${PUB_ETH} -j MASQUERADE
}

setup_iptables() {
    # NAT table
    iptables -t nat -F
    iptables -t nat -N MUSS
    # muss-server port
    iptables -t nat -A MUSS -p tcp --dport 8387 -j RETURN
    # basic private network
    iptables -t nat -A MUSS -d 172.16.0.0/16 -j RETURN
    iptables -t nat -A MUSS -d 127.0.0.0/8 -j RETURN
    iptables -t nat -A MUSS -d 169.254.0.0/16 -j RETURN
    iptables -t nat -A MUSS -d 192.168.0.0/16 -j RETURN
    iptables -t nat -A MUSS -d 224.0.0.0/4 -j RETURN
    iptables -t nat -A MUSS -d 240.0.0.0/4 -j RETURN
    # chnroute do not redirect
    iptables -t nat -A MUSS -p tcp -m set --match-set chnroute dst -j RETURN
    # redirect to muss-redir
    iptables -t nat -A MUSS -p tcp -j REDIRECT --to-ports 7070

    # set local machine can redirect
    iptables -t nat -A OUTPUT -p tcp -j MUSS
    # set router mode can redirect
    iptables -t nat -A PREROUTING -p tcp -j MUSS
}

case $1 in
    setup):
        setup_ipset
        setup_iptables
        setup_gateway_mode
        ;;
    reload-ipset):
        setup_ipset
        ;;
    reload-iptables):
        setup_iptables
        setup_gateway_mode
        ;;
    *):
        echo "redir-iptables.sh (setup|reload-ipset|reload-iptables)"
        ;;
esac
