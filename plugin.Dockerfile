# This Dockerfile builds velero-plugin

FROM golang:1.14.7 as build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GO111MODULE=on \
  GOOS=${TARGETOS} \
  GOARCH=${TARGETARCH} \
  GOARM=${TARGETVARIANT} \
  DEBIAN_FRONTEND=noninteractive \
  PATH="/root/go/bin:${PATH}"

WORKDIR /go/src/github.com/openebs/velero-plugin/

COPY go.mod go.sum ./
# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download

COPY . .

RUN make build

FROM alpine:3.11.5

ARG BUILD_DATE
ARG DBUILD_DATE
ARG DBUILD_REPO_URL
ARG DBUILD_SITE_URL

LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.name="velero-plugin"
LABEL org.label-schema.description="OpenEBS velero-plugin"
LABEL org.label-schema.build-date=$DBUILD_DATE
LABEL org.label-schema.vcs-url=$DBUILD_REPO_URL
LABEL org.label-schema.url=$DBUILD_SITE_URL

RUN mkdir /plugins
COPY --from=build /go/src/github.com/openebs/velero-plugin/_output/velero-* /plugins/
USER nobody:nobody

ENTRYPOINT ["/bin/ash", "-c", "cp /plugins/* /target/."]
