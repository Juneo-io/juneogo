Node implementation for the [Juneo Supernet](https://juneo.com) -
a multi chain network founded on the Avalanche protocol.

## Installation

The minimum computer requirements are quite modest but that as network usage increases, hardware requirements may change.

The minimum recommended hardware specification for nodes connected to Mainnet is:

- CPU: Equivalent of 4 AWS vCPU
- RAM: 8 GiB
- Storage: 500 GiB
  - Nodes running for very long periods of time or nodes with custom configurations may observe higher storage requirements.
- OS: Ubuntu 20.04/22.04 or macOS >= 12
- Network: Reliable IPv4 or IPv6 network connection, with an open public port.

If you plan to build JuneoGo from source, you will also need the following software:

- [Go](https://golang.org/doc/install) version >= 1.21.9
- [gcc](https://gcc.gnu.org/)
- g++

### Building From Source

TBD: Guide is planned to be released after open source release of both JuneoGo and JEth repositories.

### Binary Install

Download the [latest build](https://github.com/Juneo-io/juneogo-binaries).

Documentation for starting a node is available [here](https://docs.juneo.com).

For information on setting up JuneoGo using the installation scripts, please read [Set up and Connect a node using the Install Script](https://docs.juneo.com/intro/build/set-up-and-connect-a-node)

The binary to be executed is named `juneogo`.

### Docker Install

To set up JuneoGo using docker, please read [Set up and Connect a node using Docker](https://docs.juneo.com/intro/build/set-up-and-connect-a-node-with-docker)

## Running JuneoGo

### Connecting to Mainnet

To connect to the Juneo Mainnet, run the `juneogo` binary file.

You can use `Ctrl+C` to kill the node.

### Connecting to Socotra

To connect to the Socotra Testnet, run the `juneogo` binary file with the following flag:

`--network-id=socotra`

If you are using a configuration file you can set it there instead of using flags.

## Bootstrapping

A node needs to catch up to the latest network state before it can participate in consensus and serve API calls.

A node will not report healthy until it is done bootstrapping.

To check if a node is healthy you can use health API:

```bash
curl -k -H 'Content-Type: application/json' --data '{
    "jsonrpc":"2.0",
    "id"     :1,
    "method" :"health.health",
    "params": {
        "tags": ["11111111111111111111111111111111LpoYY"]
    }
}' 'http://127.0.0.1:9650/ext/health'
```

If you are using docker installation make sure to use docker ip instead of localhost.

Improvements that reduce the amount of time it takes to bootstrap are under development.

The bottleneck during bootstrapping is typically database IO. Using a more powerful CPU or increasing the database IOPS on the computer running a node will decrease the amount of time bootstrapping takes.

## Generating Code

### Running protobuf codegen

To regenerate the protobuf go code, run `scripts/protobuf_codegen.sh` from the root of the repo.

This should only be necessary when upgrading protobuf versions or modifying .proto definition files.

To use this script, you must have [buf](https://docs.buf.build/installation) (v1.31.0), protoc-gen-go (v1.33.0) and protoc-gen-go-grpc (v1.3.0) installed.

To install the buf dependencies:

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
```

If you have not already, you may need to add `$GOPATH/bin` to your `$PATH`:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

If you extract buf to ~/software/buf/bin, the following should work:

```sh
export PATH=$PATH:~/software/buf/bin/:~/go/bin
go get google.golang.org/protobuf/cmd/protoc-gen-go
go get google.golang.org/protobuf/cmd/protoc-gen-go-grpc
scripts/protobuf_codegen.sh
```

For more information, refer to the [GRPC Golang Quick Start Guide](https://grpc.io/docs/languages/go/quickstart/).

### Running mock codegen

To regenerate the [gomock](https://github.com/uber-go/mock) code, run `scripts/mock.gen.sh` from the root of the repo.

This should only be necessary when modifying exported interfaces or after modifying `scripts/mock.mockgen.txt`.
