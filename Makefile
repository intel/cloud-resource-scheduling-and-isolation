.PHONY: build test
.FORCE:

DEBUG ?= 0

GO_CMD ?= CGO_ENABLED=0 go
GO_FMT ?= gofmt

# IMAGE_VERSION := $(shell git describe --tags --dirty --always)

IMAGE_VERSION := release-v0.8.0

K8S_NAMESPACE ?= IOIsolation

SCHED_IMAGE_NAME=kube-scheduler:${IMAGE_VERSION}
NODEAGENT_IMAGE_NAME=node-agent:${IMAGE_VERSION}
AGGREGATOR_IMAGE_NAME=aggregator:${IMAGE_VERSION}
INIT_SERVICE_IMAGE_NAME=init-service:${IMAGE_VERSION}
CSI_IMAGE_NAME=localstorage-csi:${IMAGE_VERSION}


DOCKERARGS?=
# PIPARG?=
GITARG?=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
# PIPARG += --proxy $(HTTPS_PROXY)
	GITARG += -c https.proxy=$(HTTPS_PROXY)
endif


# LDFLAGS = -ldflags "-s -w -X sigs.k8s.io/IOIsolation/pkg/version.version=$(VERSION) -X sigs.k8s.io/IOIsolation/pkg/utils/hostpath.pathPrefix=$(HOSTMOUNT_PREFIX)"
ifeq ($(DEBUG),0) # Release
  # To comply with Intel's Secure Coding Standard
  # - -trimpath Replace file system paths with with abstract module path@version. golang embeds path information into generated binaries, which can reveal sensitive 
  #   information about the build environment. 
  # -ldflags="all=-s -w": Strip symbol tables and DWARF debug information.
	GOFLAGS=-ldflags="all=-s -w"
endif

REPO_HOST ?= docker.io
SCHED_IMAGE:=${REPO_HOST}/ioisolation/${SCHED_IMAGE_NAME}
NODEAGENT_IMAGE:=${REPO_HOST}/ioisolation/${NODEAGENT_IMAGE_NAME}
AGGREGATOR_IMAGE:=${REPO_HOST}/ioisolation/${AGGREGATOR_IMAGE_NAME}
INIT_SERVICE_IMAGE:=${REPO_HOST}/ioisolation/${INIT_SERVICE_IMAGE_NAME}
CSI_IMAGE:=${REPO_HOST}/ioisolation/${CSI_IMAGE_NAME}

PROTO_SOURCES = $(shell find . -name '*.proto' | grep -v /vendor/)
PROTO_GOFILES = $(patsubst %.proto,%.pb.go,$(PROTO_SOURCES))
PROTO_INCLUDE = -I$(PWD):/usr/local/include:/usr/include:$(PWD)/pkg/api/aggregator
PROTO_OPTIONS = --proto_path=. $(PROTO_INCLUDE) \
    --go_opt=paths=source_relative --go_out=. \
    --go-grpc_opt=paths=source_relative --go-grpc_out=.
PROTO_COMPILE = PATH=$(PATH):$(shell go env GOPATH)/bin; protoc $(PROTO_OPTIONS)

all: build-proto patch
	@mkdir -p bin
	$(GO_CMD) build -buildvcs=false -o bin $(GOFLAGS) ./cmd/...

scheduler:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/scheduler

node-agent:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/node-agent

aggregator:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/aggregator
csi:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/localstorage-csi

init-service:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/init-service

ioi-service:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/ioi-service

tool:
	@mkdir -p bin
	$(GO_CMD) build -o bin/kubectl-diskio $(GOFLAGS) ./cmd/kubectl-diskio

clean:
	@rm -f bin/*

build-proto: $(PROTO_GOFILES)

install:
	$(GO_CMD) install -v $(GOFLAGS) ./cmd/...

patch:
	rm -rf netlink

	git clone https://github.com/vishvananda/netlink.git $(GITARG)
	cd netlink && git checkout 5e915e0149386ce3d02379ff93f4c0a5601779d5 && git apply ../deployments/0001-adq-flower-support.patch

deploy:
	kubectl apply -f config/crd/bases/ioi.intel.com_nodestaticioinfoes.yaml
	kubectl apply -f config/crd/bases/ioi.intel.com_nodeiostatuses.yaml
	kubectl apply -f deployments/rbac/scheduler-ioi.yaml

gofmt:
	@$(GO_FMT) -w -l $$(find . -name '*.go')

gofmt-verify:
	@out=`$(GO_FMT) -w -l -d $$(find . -name '*.go')`; \
	if [ -n "$$out" ]; then \
	    echo "$$out"; \
	    exit 1; \
	fi

lint:
	golangci-lint run ./...

sdl:
	@out=`trivy fs . --skip-dirs ./vendor --skip-dirs ./netlink --ignore-unfixed`; \
	if [ -n "$$out" ]; then \
	    echo "$$out"; \
	    exit 1; \
	fi

	zip -qr IOIsolation $(PWD) -x $(PWD)/vendor/\* $(PWD)/deployments/dashboard/\* $(PWD)/.git/\* $(PWD)/checkmarx* $(PWD)/netlink/\*
	no_proxy=sast.intel.com python3 /home/applications.security.checkmarx-automation/main.py -v --domain AMR --username sys_ioi --password $(PSWD) upload --zip IOIsolation.zip --project 354838

test:
	$(GO_CMD) test ./cmd/... ./pkg/. ./pkg/agent/... ./pkg/aggregator/... ./pkg/service/... ./pkg/scheduler/...

cover:
# create code coverage report
	go test -coverprofile=coverage.data ./pkg/. ./pkg/agent/... ./pkg/aggregator/... ./pkg/service/... ./pkg/scheduler/...
	go tool cover -html=coverage.data -o ioi_coverage.html
	go tool cover -func=coverage.data -o coverage.txt

image: change_version
	docker build $(DOCKERARGS) -f ./build/scheduler/Dockerfile -t $(SCHED_IMAGE) .
	docker build $(DOCKERARGS) -f ./build/node-agent/Dockerfile -t ${NODEAGENT_IMAGE} .
	docker build $(DOCKERARGS) -f ./build/aggregator/Dockerfile -t ${AGGREGATOR_IMAGE} .
	docker build $(DOCKERARGS) -f ./build/init-service/Dockerfile -t ${INIT_SERVICE_IMAGE} .
	docker build $(DOCKERARGS) -f ./build/localstorage-csi/Dockerfile -t ${CSI_IMAGE} .

push_to_repo: change_version
	docker push $(SCHED_IMAGE)
	docker push $(NODEAGENT_IMAGE)
	docker push $(AGGREGATOR_IMAGE)
	docker push $(INIT_SERVICE_IMAGE)
	docker push $(CSI_IMAGE)

ctr: change_version
	docker save -o $(SCHED_IMAGE_NAME).tar $(SCHED_IMAGE)
	docker save -o $(NODEAGENT_IMAGE_NAME).tar $(NODEAGENT_IMAGE)
	docker save -o $(AGGREGATOR_IMAGE_NAME).tar $(AGGREGATOR_IMAGE)
	docker save -o $(INIT_SERVICE_IMAGE_NAME).tar $(INIT_SERVICE_IMAGE)
	docker save -o $(CSI_IMAGE_NAME).tar $(CSI_IMAGE)
	ctr -n=${REPO_HOST} i import $(SCHED_IMAGE_NAME).tar
	ctr -n=${REPO_HOST} i import $(NODEAGENT_IMAGE_NAME).tar
	ctr -n=${REPO_HOST} i import $(AGGREGATOR_IMAGE_NAME).tar
	ctr -n=${REPO_HOST} i import $(INIT_SERVICE_IMAGE_NAME).tar
	ctr -n=${REPO_HOST} i import $(CSI_IMAGE_NAME).tar

change_version:
	sed -i "s\SCHED_IMAGE\${SCHED_IMAGE}\g" deployments/kube-scheduler.yaml
	sed -i "s\NODEAGENT_IMAGE\${NODEAGENT_IMAGE}\g" deployments/node-agent-daemonset.yaml
	sed -i "s\AGGREGATOR_IMAGE\${AGGREGATOR_IMAGE}\g" deployments/aggregator-service.yaml
	sed -i "s\INIT_SERVICE_IMAGE\${INIT_SERVICE_IMAGE}\g" deployments/node-agent-daemonset.yaml
	sed -i "s\CSI_IMAGE\${CSI_IMAGE}\g" deployments/localstorage-csi.yaml

%.pb.go: %.proto
	$(Q)echo "Generating $@..."; \
	$(PROTO_COMPILE) $<
