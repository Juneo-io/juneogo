#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Junego root folder
JUNE_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd ) # Directory above this script

# Set the PATHS
GOPATH="$(go env GOPATH)"
jeth_path=$( cd "$JUNE_PATH"; cd ..; cd jeth && pwd)

# Where Junego binary goes
build_dir="$JUNE_PATH/build"
junego_path="$build_dir/juneogo"
plugin_dir="$build_dir/plugins"
evm_path="$plugin_dir/jevm"

# Static compilation
static_ld_flags=''
if [ "${STATIC_COMPILATION:-}" = 1 ]
then
    export CC=musl-gcc
    which $CC > /dev/null || ( echo $CC must be available for static compilation && exit 1 )
    static_ld_flags=' -extldflags "-static" -linkmode external '
fi

# Download dependencies
echo "Downloading dependencies..."
go mod download

# Build junego

# Changes to the minimum golang version must also be replicated in
# README.md
# go.mod
go_version_minimum="1.21"

go_version() {
    go version | sed -nE -e 's/[^0-9.]+([0-9.]+).+/\1/p'
}

version_lt() {
    # Return true if $1 is a lower version than than $2,
    local ver1=$1
    local ver2=$2
    # Reverse sort the versions, if the 1st item != ver1 then ver1 < ver2
    if  [[ $(echo -e -n "$ver1\n$ver2\n" | sort -rV | head -n1) != "$ver1" ]]; then
        return 0
    else
        return 1
    fi
}

if version_lt "$(go_version)" "$go_version_minimum"; then
    echo "Juneogo requires Go >= $go_version_minimum, Go $(go_version) found." >&2
    exit 1
fi

# Build with rocksdb allowed only if the environment variable ROCKSDBALLOWED is set
if [ -z ${ROCKSDBALLOWED+x} ]; then
    echo "Building Juneogo..."
    go build -ldflags "$static_ld_flags" -o "$junego_path" "$JUNE_PATH/main/"*.go
else
    echo "Building Juneogo with rocksdb enabled..."
    go build -tags rocksdballowed -ldflags "$static_ld_flags" -o "$junego_path" "$JUNE_PATH/main/"*.go
fi

# Build Coreth
echo "Building jEth..."
cd "$jeth_path"
go build -ldflags "$static_ld_flags" -o "$evm_path" "plugin/"*.go
cd "$JUNE_PATH"

# Building coreth + using go get can mess with the go.mod file.
go mod tidy

# Exit build successfully if the binaries are created
if [[ -f "$junego_path" && -f "$evm_path" ]]; then
        echo "Build Successful"
        exit 0
else
        echo "Build failure" >&2
        exit 1
fi
