IMAGE_ORG ?= abhinandan15
export IMAGE_ORG

DBUILD_DATE      = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
DBUILD_REPO_URL ?= https://github.com/openebs/velero-plugin/tree/mayastor
DBUILD_SITE_URL ?= https://openebs.io

IMAGE      = $(IMAGE_ORG)/velero-plugin-mayastor
IMAGE_TAG ?= develop

export IMAGE_TAG
export DBUILD_REPO_URL
export DBUILD_SITE_URL

DBUILD_ARGS = --build-arg DBUILD_DATE=$(DBUILD_DATE) \
              --build-arg DBUILD_REPO_URL=$(DBUILD_REPO_URL) \
              --build-arg DBUILD_SITE_URL=$(DBUILD_SITE_URL)

LINTERS ?= "goconst,gosec,unparam"

all: push-container clean

build:
	@echo ">> building binary"
	@mkdir -p _output
	CGO_ENABLED=0 go build -v -o _output/velero-plugin .

container: lint build
	@echo ">> building container"
	@cp Dockerfile _output/Dockerfile
	@cp go.mod go.sum _output/
	@cp -r pkg main.go Makefile _output/
	@sudo docker build -t $(IMAGE):$(IMAGE_TAG) $(DBUILD_ARGS) -f _output/Dockerfile _output

push-container: lint build container
	@echo ">> building container image"
	@sudo docker push $(IMAGE):$(IMAGE_TAG)

gomod:
	@echo ">> verifying go modules"
	@go mod tidy
	@go mod verify
	@git diff --exit-code -- go.sum go.mod

lint-fix:
	@echo ">> fixing gofmt"
	@gofmt -s -w .
	@echo ">> fixing goimports"
	@goimports -w .

lint: tools-check
	@echo ">> checking gofmt"
	@test -z "$$(gofmt -l .)" || (echo "❌ gofmt found unformatted files:" && gofmt -l . && exit 1)
	@echo ">> checking goimports"
	@test -z "$$(goimports -l .)" || (echo "❌ goimports found import errors:" && goimports -l . && exit 1)
	@echo ">> checking golangci-lint"
	@golangci-lint run -E $(LINTERS)

tools-check:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo ">> installing golangci-lint"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest; \
		sudo mv ./bin/golangci-lint /usr/local/bin/; \
	}
	@command -v goimports >/dev/null 2>&1 || { \
		echo ">> installing goimports"; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	}

clean:
	@rm -rf .go _output
