# Copyright 2019 The OpenEBS Authors
# Copyright 2017 the Heptio Ark contributors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

BIN = $(wildcard ark-*)

# This repo's root import path (under GOPATH).
# TODO change
REPO := github.com/openebs/ark-plugin

BUILD_IMAGE ?= gcr.io/heptio-images/golang:1.9-alpine3.6

# list only ark-plugin source code directories
PACKAGES = $(shell go list ./... | grep -v 'vendor')


# list only our .go files i.e. exlcudes any .go files from the vendor directory
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

IMAGE ?= openebs/ark-plugin

TAG ?= ci

ARCH ?= amd64

# Tools required for different make targets or for development
EXTERNAL_TOOLS=\
	gopkg.in/alecthomas/gometalinter.v1

all: $(addprefix build-, $(BIN))

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

container: all
	cp Dockerfile _output/Dockerfile
	docker build -t $(IMAGE):$(TAG) -f _output/Dockerfile _output

all-ci: $(addprefix ci-, $(BIN))

ci-%:
	$(MAKE) --no-print-directory BIN=$* ci

ci:
	mkdir -p _output
	CGO_ENABLED=0 go build -v -o _output/$(BIN) ./$(BIN)

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

# Bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "+ Installing $$tool" ; \
		go get -u $$tool; \
	done

vet:
	go vet $(GOFILES_NOVENDOR)

# Target to run gometalinter in Travis (deadcode, golint, errcheck, unconvert, goconst)
golint-travis:
	@gometalinter.v1 --install
	-gometalinter.v1 --config=metalinter.config ./...

golint:
	@gometalinter.v1 --install
	@gometalinter.v1 --vendor --disable-all -E errcheck -E misspell ./...

check: golint-travis format vet

deploy-image:
	docker push $(IMAGE):$(TAG)

clean:
	rm -rf .go _output
