FROM golang:1.23.8 AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT=""

ENV GO111MODULE=on \
  GOOS=${TARGETOS} \
  GOARCH=${TARGETARCH} \
  GOARM=${TARGETVARIANT} \
  PATH="/root/go/bin:${PATH}"

WORKDIR /go/src/github.com/openebs/velero-plugin/

RUN apt-get update && apt-get install -y make git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make build

FROM alpine:3.22.0

ARG DBUILD_DATE
ARG DBUILD_REPO_URL
ARG DBUILD_SITE_URL

LABEL org.label-schema.schema-version="1.0"
LABEL org.label-schema.name="velero-plugin-mayastor"
LABEL org.label-schema.description="OpenEBS Mayastor velero-plugin"
LABEL org.label-schema.build-date=$DBUILD_DATE
LABEL org.label-schema.vcs-url=$DBUILD_REPO_URL
LABEL org.label-schema.url=$DBUILD_SITE_URL

RUN mkdir /plugins
COPY --from=build /go/src/github.com/openebs/velero-plugin/_output/velero-* /plugins/
USER nobody:nobody

ENTRYPOINT ["/bin/sh", "-c", "cp /plugins/* /target/."]