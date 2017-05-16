#!/bin/bash
PUB_ETH=eth0
CONFIG_PATH=/etc/muss
CHNROUTE_CONFIG=${CONFIG_PATH}/chnroute.txt
CHNROUTE_PATCH=${CONFIG_PATH}/chnroute.patch
SUPERVISORD_PID_FILE=/var/run/muss-supervisord.pid

# add kernel modules for PPTP NAT
have_module=`lsmod | grep nf_nat_pptp`
if [ -z "$have_module" ]; then
    modprobe nf_nat_pptp
fi

# Optimize Kernel
optimize_kernel() {
    sysctl -w net.core.somaxconn=262144
    sysctl -w net.core.netdev_max_backlog=262144
    sysctl -w net.ipv4.tcp_max_syn_backlog=262144
    sysctl -w net.ipv4.tcp_max_orphans=262144
    sysctl -w net.netfilter.nf_conntrack_max=262144
    sysctl -w net.netfilter.nf_conntrack_tcp_timeout_established=7301
}

optimize_kernel
# End Optimize Kernel

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
    if [ -z "$have_set" ]; then
        ipset create chnroute hash:net maxelem 65536
    fi
    reload_ipset
}

clean_iptables() {
    iptables -t nat -F
    iptables -t nat -X MUSS
}

setup_gateway_mode() {
    sysctl net.ipv4.ip_forward=1
    iptables -t nat -I POSTROUTING -o ${PUB_ETH} -j MASQUERADE
}

stop_supervisor() {
    if [ -f $SUPERVISORD_PID_FILE ]; then
        kill `cat $SUPERVISORD_PID_FILE`
    fi
}

reload_supervisor() {
    if [ -f $SUPERVISORD_PID_FILE ]; then
        kill -HUP `cat $SUPERVISORD_PID_FILE`
    fi
}

start_supervisor() {
    ulimit -n 65535
    /usr/bin/supervisord -c /etc/muss/supervisord.conf
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
    start):
        setup_ipset
        setup_iptables
        setup_gateway_mode
        start_supervisor
        ;;
    start-iptables):
        setup_ipset
        setup_iptables
        setup_gateway_mode
        ;;
    start-supervisor):
        start_supervisor
        ;;
    stop):
        clean_iptables
        setup_gateway_mode
        stop_supervisor
        ;;
    reload):
        reload_supervisor
        ;;
    reload-ipset):
        setup_ipset
        ;;
    reload-iptables):
        setup_iptables
        setup_gateway_mode
        ;;
    *):
        echo "redir-iptables.sh (start|start-iptables|start-supervisor|stop|reload|reload-ipset|reload-iptables)"
        ;;
esac
