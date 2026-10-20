#!/bin/sh
# Waits for the registry, then seeds the enterprise extra-extension test catalog, extension image, and their cosign signatures.
# POSIX sh, as it also runs in the busybox-based crane image (see hack/dev/compose.yaml).
#
# Usage: hack/push-extra-extensions.sh <registry>
set -eu

registry="$1"
testdata="$(dirname "$0")/../internal/integration/testdata/extra-extensions"
repo="${registry}/extension-testing"

echo "waiting for ${registry}..."
until crane catalog --insecure "${registry}" >/dev/null 2>&1; do sleep 1; done

crane push --insecure "${testdata}/extensions-catalog-image" "${repo}/extensions:v1.13.4"
crane push --insecure "${testdata}/my-extension-image" "${repo}/my-extension:v1.0.0"
crane push --insecure "${testdata}/extensions-catalog-signature" "${repo}/extensions:$(crane digest --insecure "${repo}/extensions:v1.13.4" | sed 's/:/-/').sig"
crane push --insecure "${testdata}/my-extension-signature" "${repo}/my-extension:$(crane digest --insecure "${repo}/my-extension:v1.0.0" | sed 's/:/-/').sig"
