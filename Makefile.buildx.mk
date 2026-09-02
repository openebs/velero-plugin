# ==============================================================================
# Build Options

ifeq (${TAG}, )
	export TAG=ci
endif

# Build velero-plugin docker image with buildx
# Experimental docker feature to build cross platform multi-architecture docker images
# https://docs.docker.com/buildx/working-with-buildx/

# default list of platforms for which multiarch image is built
ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/amd64,linux/arm64"
endif

# if IMG_RESULT is unspecified, by default the image will be pushed to registry
ifeq (${IMG_RESULT}, load)
	export PUSH_ARG="--load"
	# if load is specified, image will be built only for the build machine architecture.
	export PLATFORMS="local"
else ifeq (${IMG_RESULT}, cache)
	# if cache is specified, image will only be available in the build cache, it won't be pushed or loaded
	# therefore no PUSH_ARG will be specified
else
	export PUSH_ARG="--push"
endif

# Name of the multiarch image for velero-plugin
DOCKERX_IMAGE_PLUGIN:=${IMAGE_ORG}/velero-plugin:${TAG}

.PHONY: docker.buildx.plugin
docker.buildx.plugin:
	export DOCKER_CLI_EXPERIMENTAL=enabled
	@if ! docker buildx ls | grep -q container-builder; then\
		docker buildx create --platform ${PLATFORMS} --name container-builder --use;\
	fi
	@docker buildx build --platform ${PLATFORMS} \
		-t "$(DOCKERX_IMAGE_PLUGIN)" ${DBUILD_ARGS} -f $(PWD)/plugin.Dockerfile \
		. ${PUSH_ARG}
	@echo "--> Build docker image: $(DOCKERX_IMAGE_PLUGIN)"
	@echo

.PHONY: buildx.push.plugin
buildx.push.plugin:
	BUILDX=true DIMAGE=${IMAGE_ORG}/velero-plugin ./script/buildxpush.sh

