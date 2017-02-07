#!/bin/bash
BASE_DIR=`dirname $0`
LDFLAGS="-s -w"
OSES=(linux darwin windows)
ARCHS=(amd64)
BUILD_BASE="${BASE_DIR}/build"

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
    echo Build $arch for $os to $opath
    mkdir -p $build_path
    cd 
}
