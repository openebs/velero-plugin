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

BIN = $(wildcard velero-*)

# list only velero-plugin source code directories
PACKAGES = $(shell go list ./... | grep -v 'vendor')

ARCH ?= $(shell go env GOARCH)

IMAGE = openebs/velero-plugin

# if the architecture is arm64, image name will have arm64 suffix
ifeq (${ARCH}, arm64)
	IMAGE = openebs/velero-plugin-arm64
endif

ifeq (${IMAGE_TAG}, )
	IMAGE_TAG = ci
	export IMAGE_TAG
endif

# Specify the date of build
BUILD_DATE = $(shell date +'%Y%m%d%H%M%S')

all: ci

container: all
	@cp Dockerfile _output/Dockerfile
	docker build -t $(IMAGE):$(IMAGE_TAG) --build-arg BUILD_DATE=${BUILD_DATE} -f _output/Dockerfile _output

ci:
	@mkdir -p _output
	CGO_ENABLED=0 go build -v -o _output/$(BIN) ./$(BIN)

# Run linter using docker image
lint-docker:
	@docker run -i							\
		--rm -v $$(pwd):/app -w /app 		\
		golangci/golangci-lint:v1.24.0 		\
		bash -c "GOGC=75 golangci-lint run"

# Run linter using local binary
lint:
	@golangci-lint run

test:
	@CGO_ENABLED=0 go test -v ${PACKAGES} -timeout 20m

deploy-image:
	@DIMAGE=${IMAGE} ./push

clean:
	rm -rf .go _output
