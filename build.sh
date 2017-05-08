#!/bin/bash
BASE_DIR=`pwd`/`dirname $0`
BUILD_BASE="${BASE_DIR}/build"
LDFLAGS="-s -w"

OSES=(linux darwin windows)
ARCHS=(amd64)
MODULES=(muss-server muss-local muss-proxy muss-smartdns)

build_redirect() {
    if [ ! -d ${BUILD_BASE}/redirect ]; then
        mkdir -p ${BUILD_BASE}/redirect
    fi
    cp ${BASE_DIR}/redirect-scripts/redir-iptables.sh ${BUILD_BASE}/redirect/
    cp ${BASE_DIR}/redirect-scripts/setup ${BUILD_BASE}/redirect/
    cp ${BASE_DIR}/redirect-scripts/chnroute.txt ${BUILD_BASE}/redirect/
    cat ${BASE_DIR}/redirect-scripts/chnroute.patch >> ${BUILD_BASE}/redirect/chnroute.txt
    cp ${BUILD_BASE}/linux-amd64/muss-redir ${BUILD_BASE}/redirect/
    cp ${BUILD_BASE}/linux-amd64/muss-smartdns ${BUILD_BASE}/redirect/
    cd ${BUILD_BASE}; tar zcf redirect.tar.gz redirect
}

build_target() {
    name=$1
    os=$2
    arch=$3
    build_path="${BUILD_BASE}/${os}-${arch}"
    suffix=""
    if [ "${os}" == "windows" ]; then
        suffix=".exe"
    fi
    opath="${build_path}/${name}${suffix}"
    module_path="${BASE_DIR}/cmd/${name}"
    echo Build $name for $os-${arch} to $opath
    mkdir -p $build_path
    cd $module_path; GOOS=$os GOARCH=$arch go build -ldflags "${LDFLAGS}" -o $opath
}

build_releases() {
    for os in ${OSES[@]}; do
        for arch in ${ARCHS[@]}; do
            for module in ${MODULES[@]}; do
                build_target $module $os $arch
            done
        done
    done
    build_target muss-redir linux amd64
    build_target muss-redir linux arm
}

clean_build() {
    rm -rf ${BUILD_BASE}/*
}

case $1 in
    build)
        build_releases
        ;;
    build-redirect)
        build_redirect
        ;;
    build-all)
        build_releases
        build_redirect
        ;;
    clean)
        clean_build
        ;;
    *)
        echo "build.sh (build|build-redirect|build-all|clean|help)"
        ;;
esac
