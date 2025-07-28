#!/bin/bash

### A script to copy Talos artifacts to the internal registry (air-gapped environments).
### Arguments:
###   $1 - Source registry (e.g., ghcr.io)
###   $2 - Target registry hostname (e.g., registry.example.com)
###   $3 - Talos version (e.g., v1.10.0)
###
### This script requires `crane` and `yq` to be installed. If the registry requires authentication,
### ensure that you have logged in using the docker credential store.

set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "Usage: $0 <source-registry> <target-registry> <talos-version>"
  exit 1
fi

TALOS_VERSION="$3"
SOURCE_REGISTRY="$1"
TARGET_REGISTRY="$2"

# mirror_image copies an image from the source registry to the target registry, including its signature.
#
# Arguments: image name (e.g., siderolabs/imager), tag (e.g., v1.11.0).
function mirror_image() {
    local image="$1"
    local tag="$2"
    local source_image="${SOURCE_REGISTRY}/${image}:${tag}"
    local target_image="${TARGET_REGISTRY}/${image}:${tag}"

    crane cp "${source_image}" "${target_image}"
    echo "Copied ${source_image} to ${target_image}"
    sig="$(crane digest "${source_image}" | sed 's/:/-/').sig"
    crane cp "${SOURCE_REGISTRY}/${image}:${sig}" "${TARGET_REGISTRY}/${image}:${sig}"
    echo "Copied signature of ${source_image}"
}

## Mirror base images
#
# Note: talosctl-all image is available only for Talos v1.11.0 and later (remove for earlier versions).
# Note: installer-base image is available only for Talos v1.10.0 and later, replace with 'installer' for earlier versions.

for image in \
    "siderolabs/imager" \
    "siderolabs/installer-base" \
    "siderolabs/extensions" \
    "siderolabs/overlays" \
    "siderolabs/talosctl-all" \
    ; do

    mirror_image "${image}" "${TALOS_VERSION}"
done

### Mirror extension images

catalog="siderolabs/extensions"

for image in $(crane export "${SOURCE_REGISTRY}/${catalog}:${TALOS_VERSION}" | tar x -O image-digests); do
    ## split the image reference into registry, name and tag (ghcr.io/siderolabs/i915:v1.11.0:sha256:1234567890abcdef)
    image_name=$(echo "${image}" | cut -d: -f1)
    image_tag=$(echo "${image}" | cut -d: -f2-)
    ## trim the source registry from the image name
    image_name=$(echo "${image_name}" | sed "s|${SOURCE_REGISTRY}/||")

    echo "Processing extension image: ${image_name} ${image_tag}"

    ## mirror the image
    mirror_image "${image_name}" "${image_tag}"
done

### Mirror overlay images

catalog="siderolabs/overlays"

for image in $(crane export "${SOURCE_REGISTRY}/${catalog}:${TALOS_VERSION}" | tar x -O overlays.yaml | yq '.overlays[] | .image + "@" + .digest' | sort -u); do
    ## split the image reference into registry, name and tag (ghcr.io/siderolabs/i915:v1.11.0:sha256:1234567890abcdef)
    image_name=$(echo "${image}" | cut -d: -f1)
    image_tag=$(echo "${image}" | cut -d: -f2-)
    ## trim the source registry from the image name
    image_name=$(echo "${image_name}" | sed "s|${SOURCE_REGISTRY}/||")

    echo "Processing overlay image: ${image_name} ${image_tag}"

    ## mirror the image
    mirror_image "${image_name}" "${image_tag}"
done
