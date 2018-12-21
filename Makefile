BINS = $(wildcard ark-*)

REPO ?= github.com/mynktl/openebs-ark-plugin

BUILD_IMAGE ?= gcr.io/heptio-images/golang:1.9-alpine3.6

IMAGE ?= openebs/ark-plugin

TAG ?= latest

ARCH ?= amd64

ifndef PKGS
PKGS := $(shell go list ./... 2>&1 | grep -v 'github.com/mynktl/openebs-ark-plugin/vendor')
endif

all: $(addprefix build-, $(BINS))

build-%:
	$(MAKE) --no-print-directory BIN=$* build

build: _output/$(BIN)

_output/$(BIN): $(BIN)/*.go
	mkdir -p .go/src/$(REPO) .go/pkg .go/std/$(ARCH) _output
	docker run \
				 --rm \
				 -u $$(id -u):$$(id -g) \
				 -v $$(pwd)/.go/pkg:/go/pkg \
				 -v $$(pwd)/.go/src:/go/src \
				 -v $$(pwd)/.go/std:/go/std \
				 -v $$(pwd):/go/src/$(REPO) \
				 -v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static \
				 -e CGO_ENABLED=0 \
				 -w /go/src/$(REPO) \
				 $(BUILD_IMAGE) \
				 go build -installsuffix "static" -i -v -o _output/$(BIN) ./$(BIN)

lint:
	go get -v github.com/golang/lint/golint
	for file in $$(find . -name '*.go' | grep -v vendor | grep -v '\.pb\.go' | grep -v '\.pb\.gw\.go'); do \
		golint $${file}; \
		if [ -n "$$(golint $${file})" ]; then \
			exit 1; \
		fi; \
	done

vet:
	go vet $(PKGS)

errcheck:
	go get -v github.com/kisielk/errcheck
	errcheck -verbose -blank $(PKGS)

check: lint errcheck vet

container: all
	cp Dockerfile _output/Dockerfile
	docker build -t $(IMAGE):$(TAG) -f _output/Dockerfile _output

deploy: 
	docker push $(IMAGE):$(TAG)

clean:
	rm -rf .go _output
