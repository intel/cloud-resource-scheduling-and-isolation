GO_CMD ?= CGO_ENABLED=0 go

REPO_HOST ?= localhost:5000
IODRIVER_IMAGE=${REPO_HOST}/iodriver:alpha

DOCKERARGS?=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

.PHONY: all
all: iodriver

.PHONY: iodriver
iodriver:
	@mkdir -p bin
	$(GO_CMD) build -o bin $(GOFLAGS) ./cmd/iodriver

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: image
image: clean
	docker build $(DOCKERARGS) -f ./build/Dockerfile -t $(IODRIVER_IMAGE) .	

.PHONY: push_image
push_image: clean
	docker push $(IODRIVER_IMAGE)