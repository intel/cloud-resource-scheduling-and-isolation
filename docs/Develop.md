## For Dev
### install protoc

```
PB_REL="https://github.com/protocolbuffers/protobuf/releases"
curl -LO $PB_REL/download/v3.15.8/protoc-3.15.8-linux-x86_64.zip
unzip protoc-3.15.8-linux-x86_64.zip -d $HOME/.local
export PATH="$PATH:$HOME/.local/bin"
```
### install go plugins for grpc

```
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
```

### update PATH

```
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Compile
On **the master node**, build the binaries, container images and push the images to the local registry.  
#### Install Build Dependencies 
Execute the following command to install build dependencies
```
sudo apt install build-essential docker-ce
```
Apply the netlink patch deployments/0001-adq-flower-support.patch before build binary.
You must execute below command when you **first compile** project
```
make patch
```
If you need to specify https proxy,
```
make patch HTTPS_PROXY=<YOUR_HTTPS_PROXY>
```
Once installed, make sure that the http_proxy and https_proxy is configured properly for docker if you are behind an enterprise proxy. 
Replace `127.0.0.1:5000` with the local registry (eg. localhost:5000).
```
$ make
$ make image REPO_HOST=127.0.0.1:5000/
$ make push_to_repo REPO_HOST=127.0.0.1:5000/
```

## Prerequisites
### Component Versions
The project is tested and developed with the cloud native components of the following versions. It is **highly recommended** to use these specified versions.    
| Component | Version | 
| ------------- | ------------- |
| Kubernetes | 1.29.9 |
| Containerd | 1.7.0 |
| Runc | 1.1.4 |
| Ubuntu | 22.04 |
| Go | 1.23.0 | 
| openssl | 3.0.2 | 


## Test

### Unit test

```
$ cd IOIsolation/
$ make test
```

## Debug

### open info and debug log for disk ioi service


open info log

```
$ ./bin/ioi-disk-io-service -v 3
```
open debug and info log

```
$./bin/ioi-disk-io-service -v 4
```

### lint check
install binary will be $(go env GOPATH)/bin/golangci-lint

```
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.53.3
export PATH=$PATH:$GOPATH/bin
golangci-lint --version
golangci-lint run ./...
```

use it

```
make lint
```

### sdl check
install sdl tool

```
https://aquasecurity.github.io/trivy/v0.43/getting-started/installation/
```

use it

```
make sdl
```
