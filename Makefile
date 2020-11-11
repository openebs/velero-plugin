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


# The images can be pushed to any docker/image registeries
# like docker hub, quay. The registries are specified in
# the script https://raw.githubusercontent.com/openebs/charts/gh-pages/scripts/release/buildscripts/push.
#
# The images of a project or company can then be grouped
# or hosted under a unique organization key like `openebs`
#
# Each component (container) will be pushed to a unique
# repository under an organization.
# Putting all this together, an unique uri for a given
# image comprises of:
#   <registry url>/<image org>/<image repo>:<image-tag>
#
# IMAGE_ORG can be used to customize the organization
# under which images should be pushed.
# By default the organization name is `openebs`.

ifeq (${IMAGE_ORG}, )
  IMAGE_ORG="openebs"
  export IMAGE_ORG
endif

# Specify the date of build
DBUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Specify the docker arg for repository url
ifeq (${DBUILD_REPO_URL}, )
  DBUILD_REPO_URL="https://github.com/openebs/velero-plugin"
  export DBUILD_REPO_URL
endif

# Specify the docker arg for website url
ifeq (${DBUILD_SITE_URL}, )
  DBUILD_SITE_URL="https://openebs.io"
  export DBUILD_SITE_URL
endif

export DBUILD_ARGS=--build-arg DBUILD_DATE=${DBUILD_DATE} --build-arg DBUILD_REPO_URL=${DBUILD_REPO_URL} --build-arg DBUILD_SITE_URL=${DBUILD_SITE_URL}

IMAGE = ${IMAGE_ORG}/velero-plugin-amd64

# if the architecture is arm64, image name will have arm64 suffix
ifeq (${ARCH}, arm64)
	IMAGE = ${IMAGE_ORG}/velero-plugin-arm64
endif

ifeq (${IMAGE_TAG}, )
	IMAGE_TAG = ci
	export IMAGE_TAG
endif

# Specify the date of build
BUILD_DATE = $(shell date +'%Y%m%d%H%M%S')

#List of linters used by docker lint and local lint
LINTERS ?= "goconst,gofmt,goimports,gosec,unparam"

all: build

container: all
	@echo ">> building container"
	@cp Dockerfile _output/Dockerfile
	@sudo docker build -t $(IMAGE):$(IMAGE_TAG) ${DBUILD_ARGS} -f _output/Dockerfile _output

build:
	@echo ">> building binary"
	@mkdir -p _output
	CGO_ENABLED=0 go build -v -o _output/$(BIN) ./$(BIN)

gomod: ## Ensures fresh go.mod and go.sum.
	@echo ">> verifying go modules"
	@go mod tidy
	@go mod verify
	@git diff --exit-code -- go.sum go.mod

# Run linter using docker image
lint-docker: gomod
	@echo ">> running golangci-lint"
	@sudo docker run -i	\
		--rm -v $$(pwd):/app -w /app	\
		golangci/golangci-lint:v1.24.0	\
		bash -c "GOGC=75 golangci-lint run -E $(LINTERS)"

# Run linter using local binary
lint: gomod
	@echo ">> running golangci-lint"
	@golangci-lint run -E $(LINTERS)

test:
	@CGO_ENABLED=0 go test -v ${PACKAGES} -timeout 20m

deploy-image:
	@curl --fail --show-error -s  https://raw.githubusercontent.com/openebs/charts/gh-pages/scripts/release/buildscripts/push > ./push
	@chmod +x ./push
	@DIMAGE=${IMAGE} ./push

clean:
	rm -rf .go _output


.PHONY: check-license
check-license:
	@echo ">> checking license header"
	@licRes=$$(for file in $$(find . -type f -iname '*.go' ! -path './vendor/*' ! -path './pkg/debug/*' ) ; do \
				awk 'NR<=3' $$file | grep -Eq "(Copyright|generated|GENERATED)" || echo $$file; \
				done); \
			if [ -n "$${licRes}" ]; then \
				echo "license header checking failed:"; echo "$${licRes}"; \
				exit 1; \
			fi

include Makefile.buildx.mk
