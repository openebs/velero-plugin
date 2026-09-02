# This Dockerfile builds velero-plugin

FROM alpine:3.11.5
RUN mkdir /plugins
ADD velero-* /plugins/
USER nobody:nobody

ARG ARCH
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

ENTRYPOINT ["/bin/ash", "-c", "cp /plugins/* /target/."]
