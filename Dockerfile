FROM alpine:3.6
RUN mkdir /plugins
ADD ark-* /plugins/
USER nobody:nobody

ARG BUILD_DATE
LABEL org.label-schema.build-date=$BUILD_DATE

ENTRYPOINT ["/bin/ash", "-c", "cp /plugins/* /target/."]
