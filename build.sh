#!/bin/bash
BASE_DIR=`pwd`/`dirname $0`
BUILD_BASE="${BASE_DIR}/build"
LDFLAGS="-s -w"

OSES=(linux darwin windows)
ARCHS=(amd64)
MODULES=(muss-server muss-local muss-proxy)

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
}

clean_build() {
    rm -rf ${BUILD_BASE}/*
}

case $1 in
    build)
        build_releases
        ;;
    clean)
        clean_build
        ;;
    help)
        echo "build.sh (build|clean|help)"
        ;;
    *)
        build_releases
        ;;
esac
