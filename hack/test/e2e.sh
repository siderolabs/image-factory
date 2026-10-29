#!/usr/bin/env bash
# Runs the Playwright e2e specs (frontend/e2e) against an Image Factory started from an image, see the e2e make targets.
#
# Variables:
#   IMAGE                  the Image Factory image to test
#   PROJECT                the Playwright project: community, or enterprise or auth0 for an enterprise build
#   CORE_REGISTRY          the registry serving the Talos images
#   REGISTRY_MIRROR_ADDR   a mirror of CORE_REGISTRY to use instead; without one, a pull-through cache is started
#   TEST_OUTPUTS_DIR       where the factory log and the Playwright report and results go
#
# The auth0 project also needs the Auth0 tenant and its test user:
#   AUTH0_DOMAIN, AUTH0_AUDIENCE, AUTH0_CLIENT_ID, AUTH0_CLIENT_SECRET, AUTH0_USERNAME, AUTH0_PASSWORD
set -eoux pipefail

IMAGE="${IMAGE:?}"
PROJECT="${PROJECT:-community}"
CORE_REGISTRY="${CORE_REGISTRY:-ghcr.io}"
REGISTRY_MIRROR_ADDR="${REGISTRY_MIRROR_ADDR:-}"
TEST_OUTPUTS_DIR="${TEST_OUTPUTS_DIR:-${PWD}/_out/e2e}"

FACTORY_CONTAINER_NAME=e2e-image-factory
REGISTRY_CONTAINER_NAME=local-if
PROXY_CONTAINER_NAME=local-if-proxy
# Matches http.externalURL in hack/test/e2e/config.yaml.
BASE_URL=http://localhost:8080

mkdir -p "${TEST_OUTPUTS_DIR}"

# Not in /tmp: CI's Docker daemon shares the workspace with the runner, but not its /tmp.
WORK_DIR="$(mktemp -d -p "${TEST_OUTPUTS_DIR}")"

function cleanup() {
  docker logs "${FACTORY_CONTAINER_NAME}" 2>&1 | tee "${TEST_OUTPUTS_DIR}/image-factory.log" || true
  docker rm -f "${FACTORY_CONTAINER_NAME}" "${REGISTRY_CONTAINER_NAME}" "${PROXY_CONTAINER_NAME}" || true
  rm -rf "${WORK_DIR}"
}

trap cleanup EXIT SIGINT

docker pull "${IMAGE}"

# The registry hack/test/e2e/config.yaml stores the factory's own artifacts in.
docker rm -f "${REGISTRY_CONTAINER_NAME}" || true
docker run -d -p 5100:5000 --name="${REGISTRY_CONTAINER_NAME}" registry:3

# Without a mirror, back the Talos images with a pull-through cache.
docker rm -f "${PROXY_CONTAINER_NAME}" || true

if [[ -z "${REGISTRY_MIRROR_ADDR}" ]]; then
  docker run -d -p 5000:5000 --name="${PROXY_CONTAINER_NAME}" -e REGISTRY_PROXY_REMOTEURL="https://${CORE_REGISTRY}" ghcr.io/distribution/distribution:edge
  REGISTRY_MIRROR_ADDR=127.0.0.1:5000
fi

# A fresh key, so no asset cached by an earlier run is used.
openssl ecparam -name prime256v1 -genkey -noout -out "${WORK_DIR}/cache-signing-key.key"
chmod 0644 "${WORK_DIR}/cache-signing-key.key"

CONFIG_ARGS=(--config=/etc/image-factory/config.yaml)
MOUNTS=(
  -v "${PWD}/hack/test/e2e/config.yaml:/etc/image-factory/config.yaml:ro"
  -v "${WORK_DIR}/cache-signing-key.key:/etc/image-factory/cache-signing-key.key:ro"
)
FACTORY_ENV=()
PLAYWRIGHT_ENV=()

if [[ "${PROJECT}" == "enterprise" || "${PROJECT}" == "auth0" ]]; then
  CONFIG_ARGS+=(--config=/etc/image-factory/config.enterprise.yaml)
  MOUNTS+=(
    -v "${PWD}/hack/test/e2e/config.enterprise.yaml:/etc/image-factory/config.enterprise.yaml:ro"
    -v "${PWD}/hack/dev/htpasswd:/etc/image-factory/htpasswd:ro"
  )
fi

if [[ "${PROJECT}" == "enterprise" ]]; then
  # The credentials in hack/dev/htpasswd.
  PLAYWRIGHT_ENV+=(-e AUTH_USERNAME=developer -e AUTH_PASSWORD=SideroTest)
fi

if [[ "${PROJECT}" == "auth0" ]]; then
  CONFIG_ARGS+=(--config=/etc/image-factory/config.auth0.yaml)
  MOUNTS+=(-v "${PWD}/hack/test/e2e/config.auth0.yaml:/etc/image-factory/config.auth0.yaml:ro")

  # Exported and passed by name, untraced, so the secrets stay out of the log.
  set +x
  export IF_AUTHENTICATION_AUTH0_DOMAIN="${AUTH0_DOMAIN:?}"
  export IF_AUTHENTICATION_AUTH0_AUDIENCE="${AUTH0_AUDIENCE:?}"
  export IF_AUTHENTICATION_AUTH0_CLIENTID="${AUTH0_CLIENT_ID:?}"
  export IF_AUTHENTICATION_AUTH0_CLIENTSECRET="${AUTH0_CLIENT_SECRET:?}"
  IF_AUTHENTICATION_AUTH0_SESSIONKEY="$(openssl rand -base64 32)"
  export IF_AUTHENTICATION_AUTH0_SESSIONKEY
  export AUTH0_USERNAME="${AUTH0_USERNAME:?}" AUTH0_PASSWORD="${AUTH0_PASSWORD:?}"
  set -x

  FACTORY_ENV+=(
    -e IF_AUTHENTICATION_AUTH0_DOMAIN
    -e IF_AUTHENTICATION_AUTH0_AUDIENCE
    -e IF_AUTHENTICATION_AUTH0_CLIENTID
    -e IF_AUTHENTICATION_AUTH0_CLIENTSECRET
    -e IF_AUTHENTICATION_AUTH0_SESSIONKEY
  )
  PLAYWRIGHT_ENV+=(-e AUTH0_USERNAME -e AUTH0_PASSWORD)
fi

# Environment variables apply after the config files, so they win.
CONFIG_ARGS+=(--config=env=IF_)

docker rm -f "${FACTORY_CONTAINER_NAME}" || true
docker run -d --name "${FACTORY_CONTAINER_NAME}" \
  --net=host --cap-drop=all --cap-add=DAC_OVERRIDE --userns=host \
  -e IF_ARTIFACTS_CORE_REGISTRY="${REGISTRY_MIRROR_ADDR}" \
  "${FACTORY_ENV[@]}" \
  "${MOUNTS[@]}" \
  "${IMAGE}" \
  "${CONFIG_ARGS[@]}"

# Not /readyz: in an enterprise build it waits for the vulnerability scanner's database, which the specs don't use.
timeout 120 bash -c "until curl -sf -o /dev/null ${BASE_URL}/healthz; do sleep 1; done"

# The e2e specs run in a container with Playwright's browsers and their dependencies.
cd frontend/

docker buildx build --load . -t image-factory-e2e
docker run --rm \
  --net=host \
  -e CI="${CI:-}" \
  -e BASE_URL="${BASE_URL}" \
  -e PROJECT="${PROJECT}" \
  "${PLAYWRIGHT_ENV[@]}" \
  -v "${TEST_OUTPUTS_DIR}/playwright-report:/tmp/test/playwright-report" \
  -v "${TEST_OUTPUTS_DIR}/test-results:/tmp/test/test-results" \
  image-factory-e2e
