#!/bin/bash

set -e

if [ -z "${DIMAGE}" ];
then
  echo "Error: DIMAGE is not specified";
  exit 1
fi

function pushBuildx() {
  BUILD_TAG="latest"
  TARGET_IMG=${DIMAGE}

# TODO Currently ci builds with commit tag will not be generated,
# since buildx does not support multiple repo
  # if not a release build set the tag and ci image
  if [ -z "${RELEASE_TAG}" ]; then
    return
#    BUILD_ID=$(git describe --tags --always)
#    BUILD_TAG="${BRANCH}-${BUILD_ID}"
#    TARGET_IMG="${DIMAGE}-ci"
  fi

  echo "Tagging and pushing ${DIMAGE}:${TAG} as ${TARGET_IMG}:${BUILD_TAG}"
  docker buildx imagetools create "${DIMAGE}:${TAG}" -t "${TARGET_IMG}:${BUILD_TAG}"
}

# if the push is for a buildx build
if [[ ${BUILDX} ]]; then
  pushBuildx
  exit 0
fi
