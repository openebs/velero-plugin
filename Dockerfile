FROM alpine:3.6
RUN mkdir /plugins
ADD ark-* /plugins/
USER nobody:nobody
ENTRYPOINT ["/bin/ash", "-c", "cp /plugins/* /target/."]
