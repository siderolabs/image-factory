# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2026-01-28T15:13:10Z by kres edff623.

# common variables

SHA := $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG := $(shell git describe --tag --always --dirty --match v[0-9]\*)
TAG_SUFFIX ?=
ABBREV_TAG := $(shell git describe --tags >/dev/null 2>/dev/null && git describe --tag --always --match v[0-9]\* --abbrev=0 || echo 'undefined')
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ARTIFACTS := _out
IMAGE_TAG ?= $(TAG)$(TAG_SUFFIX)
OPERATING_SYSTEM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
WITH_DEBUG ?= false
WITH_RACE ?= false
REGISTRY ?= ghcr.io
USERNAME ?= siderolabs
REGISTRY_AND_USERNAME ?= $(REGISTRY)/$(USERNAME)
PROTOBUF_GO_VERSION ?= 1.36.11
GRPC_GO_VERSION ?= 1.6.0
GRPC_GATEWAY_VERSION ?= 2.27.4
VTPROTOBUF_VERSION ?= 0.6.0
GOIMPORTS_VERSION ?= 0.41.0
GOMOCK_VERSION ?= 0.6.0
DEEPCOPY_VERSION ?= v0.5.8
GOLANGCILINT_VERSION ?= v2.8.0
GOFUMPT_VERSION ?= v0.9.2
GO_VERSION ?= 1.25.6
GO_BUILDFLAGS ?=
GO_BUILDTAGS ?= ,
GO_LDFLAGS ?=
CGO_ENABLED ?= 0
GOTOOLCHAIN ?= local
GOEXPERIMENT ?=
GO_BUILDFLAGS += -tags $(GO_BUILDTAGS)
TESTPKGS ?= ./...
HELMREPO ?= $(REGISTRY)/$(USERNAME)/charts
COSIGN_ARGS ?=
HELMDOCS_VERSION ?= v1.14.2
KRES_IMAGE ?= ghcr.io/siderolabs/kres:latest
CONFORMANCE_IMAGE ?= ghcr.io/siderolabs/conform:latest

# docker build settings

BUILD := docker buildx build
PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false
CI_ARGS ?=
WITH_BUILD_DEBUG ?=
BUILDKIT_MULTI_PLATFORM ?=
COMMON_ARGS = --file=Dockerfile
COMMON_ARGS += --provenance=false
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --build-arg=BUILDKIT_MULTI_PLATFORM=$(BUILDKIT_MULTI_PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=ARTIFACTS="$(ARTIFACTS)"
COMMON_ARGS += --build-arg=SHA="$(SHA)"
COMMON_ARGS += --build-arg=TAG="$(TAG)"
COMMON_ARGS += --build-arg=ABBREV_TAG="$(ABBREV_TAG)"
COMMON_ARGS += --build-arg=USERNAME="$(USERNAME)"
COMMON_ARGS += --build-arg=REGISTRY="$(REGISTRY)"
COMMON_ARGS += --build-arg=TOOLCHAIN="$(TOOLCHAIN)"
COMMON_ARGS += --build-arg=CGO_ENABLED="$(CGO_ENABLED)"
COMMON_ARGS += --build-arg=GO_BUILDFLAGS="$(GO_BUILDFLAGS)"
COMMON_ARGS += --build-arg=GO_LDFLAGS="$(GO_LDFLAGS)"
COMMON_ARGS += --build-arg=GOTOOLCHAIN="$(GOTOOLCHAIN)"
COMMON_ARGS += --build-arg=GOEXPERIMENT="$(GOEXPERIMENT)"
COMMON_ARGS += --build-arg=PROTOBUF_GO_VERSION="$(PROTOBUF_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GO_VERSION="$(GRPC_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GATEWAY_VERSION="$(GRPC_GATEWAY_VERSION)"
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)"
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)"
COMMON_ARGS += --build-arg=GOMOCK_VERSION="$(GOMOCK_VERSION)"
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)"
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)"
COMMON_ARGS += --build-arg=GOFUMPT_VERSION="$(GOFUMPT_VERSION)"
COMMON_ARGS += --build-arg=TESTPKGS="$(TESTPKGS)"
COMMON_ARGS += --build-arg=HELMDOCS_VERSION="$(HELMDOCS_VERSION)"
COMMON_ARGS += --build-arg=PKGS_PREFIX="$(PKGS_PREFIX)"
COMMON_ARGS += --build-arg=PKGS="$(PKGS)"
TOOLCHAIN ?= docker.io/golang:1.25-alpine

# extra variables

PKGS_PREFIX ?= ghcr.io/siderolabs
PKGS ?= v1.13.0-alpha.0-40-g553e0fb
RUN_TESTS_DIRECT ?= TestIntegrationDirect
TEST_FLAGS ?=
RUN_TESTS_S3 ?= TestIntegrationS3
RUN_TESTS_CDN ?= TestIntegrationCDN
RUN_TESTS_PROXY ?= TestIntegrationDirect
RUN_TESTS_ENTERPRISE ?= TestIntegrationDirect
GOOS ?= $(shell uname -s | tr "[:upper:]" "[:lower:]")
TALOS_VERSION ?= 1.12.2
K8S_VERSION ?= v1.35.0
CHART_VERSION ?= 0.0.0-alpha.0

# help menu

export define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- docker (19.03 or higher)

## Creating a Builder Instance

The build process makes use of experimental Docker features (buildx).
To enable experimental features, add 'experimental: "true"' to '/etc/docker/daemon.json' on
Linux or enable experimental features in Docker GUI for Windows or Mac.

To create a builder instance, run:

	docker buildx create --name local --use

If running builds that needs to be cached aggresively create a builder instance with the following:

	docker buildx create --name local --use --config=config.toml

config.toml contents:

[worker.oci]
  gc = true
  gckeepstorage = 50000

  [[worker.oci.gcpolicy]]
    keepBytes = 10737418240
    keepDuration = 604800
    filters = [ "type==source.local", "type==exec.cachemount", "type==source.git.checkout"]
  [[worker.oci.gcpolicy]]
    all = true
    keepBytes = 53687091200

If you already have a compatible builder instance, you may use that instead.

## Artifacts

All artifacts will be output to ./$(ARTIFACTS). Images will be tagged with the
registry "$(REGISTRY)", username "$(USERNAME)", and a dynamic tag (e.g. $(IMAGE):$(IMAGE_TAG)).
The registry and username can be overridden by exporting REGISTRY, and USERNAME
respectively.

endef

ifneq (, $(filter $(WITH_BUILD_DEBUG), t true TRUE y yes 1))
BUILD := BUILDX_EXPERIMENTAL=1 docker buildx debug --invoke /bin/sh --on error build
endif

ifneq (, $(filter $(WITH_RACE), t true TRUE y yes 1))
GO_BUILDFLAGS += -race
CGO_ENABLED := 1
GO_LDFLAGS += -linkmode=external -extldflags '-static'
endif

ifneq (, $(filter $(WITH_DEBUG), t true TRUE y yes 1))
GO_BUILDTAGS := $(GO_BUILDTAGS)sidero.debug,
else
GO_LDFLAGS += -s
endif

ifneq (, $(filter $(WITH_ENTERPRISE), t true TRUE y yes 1))
GO_BUILDTAGS := $(GO_BUILDTAGS)enterprise,
endif

all: unit-tests image-factory image-image-factory helm lint

$(ARTIFACTS):  ## Creates artifacts directory.
	@mkdir -p $(ARTIFACTS)

.PHONY: clean
clean:  ## Cleans up all artifacts.
	@rm -rf $(ARTIFACTS)

target-%:  ## Builds the specified target defined in the Dockerfile. The build result will only remain in the build cache.
	@$(BUILD) --target=$* $(COMMON_ARGS) $(TARGET_ARGS) $(CI_ARGS) .

registry-%:  ## Builds the specified target defined in the Dockerfile and the output is an image. The image is pushed to the registry if PUSH=true.
	@$(MAKE) target-$* TARGET_ARGS="--tag=$(REGISTRY)/$(USERNAME)/$(IMAGE_NAME):$(IMAGE_TAG)" BUILDKIT_MULTI_PLATFORM=1

local-%:  ## Builds the specified target defined in the Dockerfile using the local output type. The build result will be output to the specified local destination.
	@$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)"
	@PLATFORM=$(PLATFORM) DEST=$(DEST) bash -c '\
	  for platform in $$(tr "," "\n" <<< "$$PLATFORM"); do \
	    directory="$${platform//\//_}"; \
	    if [[ -d "$$DEST/$$directory" ]]; then \
		  echo $$platform; \
	      mv -f "$$DEST/$$directory/"* $$DEST; \
	      rmdir "$$DEST/$$directory/"; \
	    fi; \
	  done'

.PHONY: check-dirty
check-dirty:
	@if test -n "`git status --porcelain`"; then echo "Source tree is dirty"; git status; git diff; exit 1 ; fi

generate:  ## Generate .proto definitions.
	@$(MAKE) local-$@ DEST=./
	@sed -i "s/appVersion: .*/appVersion: \"$$(cat internal/version/data/tag)\"/" deploy/helm/image-factory/Chart.yaml

lint-golangci-lint:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-golangci-lint-fmt:  ## Runs golangci-lint formatter and tries to fix issues automatically.
	@$(MAKE) local-$@ DEST=.

lint-gofumpt:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

.PHONY: fmt
fmt:  ## Formats the source code
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) \
		bash -c "export GOTOOLCHAIN=local; \
		export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; \
		go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION) && \
		gofumpt -w ."

lint-govulncheck:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

.PHONY: base
base:  ## Prepare base toolchain
	@$(MAKE) target-$@

.PHONY: unit-tests
unit-tests:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: unit-tests-race
unit-tests-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@

.PHONY: integration-direct
integration-direct: integration.test
	@$(MAKE) image-image-factory PUSH=true
	docker pull $(REGISTRY)/$(USERNAME)/image-factory:$(TAG)
	docker rm -f local-if || true
	docker run -d -p 5100:5000 --name=local-if registry:3
	docker run --rm --net=host -v /var/run:/var/run -v $(PWD)/$(ARTIFACTS)/:/out/ -v $(PWD)/$(ARTIFACTS)/integration.test:/bin/integration.test:ro --entrypoint /bin/integration.test $(REGISTRY)/$(USERNAME)/image-factory:$(TAG) -test.v $(TEST_FLAGS) -test.coverprofile=/out/coverage-integration-direct.txt -test.run $(RUN_TESTS_DIRECT)
	docker rm -f local-if

.PHONY: integration-s3
integration-s3: integration.test
	@$(MAKE) image-image-factory PUSH=true
	docker pull $(REGISTRY)/$(USERNAME)/image-factory:$(TAG)
	docker rm -f local-if || true
	docker run -d -p 5100:5000 --name=local-if registry:3
	docker run --rm --net=host -v /var/run:/var/run -v $(PWD)/$(ARTIFACTS)/:/out/ -v $(PWD)/$(ARTIFACTS)/integration.test:/bin/integration.test:ro --entrypoint /bin/integration.test $(REGISTRY)/$(USERNAME)/image-factory:$(TAG) -test.v $(TEST_FLAGS) -test.coverprofile=/out/coverage-integration-s3.txt -test.run $(RUN_TESTS_S3)
	docker rm -f local-if

.PHONY: integration-cdn
integration-cdn: integration.test
	@$(MAKE) image-image-factory PUSH=true
	docker pull $(REGISTRY)/$(USERNAME)/image-factory:$(TAG)
	docker rm -f local-if || true
	docker run -d -p 5100:5000 --name=local-if registry:3
	docker run --rm --net=host -v /var/run:/var/run -v $(PWD)/$(ARTIFACTS)/:/out/ -v $(PWD)/$(ARTIFACTS)/integration.test:/bin/integration.test:ro --entrypoint /bin/integration.test $(REGISTRY)/$(USERNAME)/image-factory:$(TAG) -test.v $(TEST_FLAGS) -test.coverprofile=/out/coverage-integration-cdn.txt -test.run $(RUN_TESTS_CDN)
	docker rm -f local-if

.PHONY: integration-proxy-installer
integration-proxy-installer: integration.test
	@$(MAKE) image-image-factory PUSH=true
	docker pull $(REGISTRY)/$(USERNAME)/image-factory:$(TAG)
	docker rm -f local-if || true
	docker run -d -p 5100:5000 --name=local-if registry:3
	docker run --rm --net=host -v /var/run:/var/run -v $(PWD)/$(ARTIFACTS)/:/out/ -v $(PWD)/$(ARTIFACTS)/integration.test:/bin/integration.test:ro --entrypoint /bin/integration.test $(REGISTRY)/$(USERNAME)/image-factory:$(TAG) -test.v $(TEST_FLAGS) -test.coverprofile=/out/coverage-integration-direct.txt -test.run $(RUN_TESTS_PROXY)
	docker rm -f local-if

.PHONY: integration-enterprise
integration-enterprise: integration.enterprise.test
	@$(MAKE) image-image-factory PUSH=true
	docker pull $(REGISTRY)/$(USERNAME)/image-factory:$(TAG)
	docker rm -f local-if || true
	docker run -d -p 5100:5000 --name=local-if registry:3
	docker run --rm --net=host -v /var/run:/var/run -v $(PWD)/$(ARTIFACTS)/:/out/ -v $(PWD)/$(ARTIFACTS)/integration.enterprise.test:/bin/integration.test:ro --entrypoint /bin/integration.test $(REGISTRY)/$(USERNAME)/image-factory:$(TAG) -test.v $(TEST_FLAGS) -test.coverprofile=/out/coverage-integration-enterprise.txt -test.run $(RUN_TESTS_ENTERPRISE)
	docker rm -f local-if

.PHONY: $(ARTIFACTS)/image-factory-linux-amd64
$(ARTIFACTS)/image-factory-linux-amd64:
	@$(MAKE) local-image-factory-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: image-factory-linux-amd64
image-factory-linux-amd64: $(ARTIFACTS)/image-factory-linux-amd64  ## Builds executable for image-factory-linux-amd64.

.PHONY: $(ARTIFACTS)/image-factory-linux-arm64
$(ARTIFACTS)/image-factory-linux-arm64:
	@$(MAKE) local-image-factory-linux-arm64 DEST=$(ARTIFACTS)

.PHONY: image-factory-linux-arm64
image-factory-linux-arm64: $(ARTIFACTS)/image-factory-linux-arm64  ## Builds executable for image-factory-linux-arm64.

.PHONY: image-factory
image-factory: image-factory-linux-amd64 image-factory-linux-arm64  ## Builds executables for image-factory.

.PHONY: lint-markdown
lint-markdown:  ## Runs markdownlint.
	@$(MAKE) target-$@

.PHONY: lint
lint: lint-golangci-lint lint-gofumpt lint-govulncheck lint-markdown  ## Run all linters for the project.

.PHONY: lint-fmt
lint-fmt: lint-golangci-lint-fmt  ## Run all linter formatters and fix up the source tree.

.PHONY: image-image-factory
image-image-factory: tailwind  ## Builds image for image-factory.
	@$(MAKE) registry-$@ IMAGE_NAME="image-factory"

.PHONY: helm
helm:  ## Package helm chart
	@helm package deploy/helm/image-factory -d $(ARTIFACTS)

.PHONY: helm-release
helm-release: helm  ## Release helm chart
	@helm push $(ARTIFACTS)/image-factory-*.tgz oci://$(HELMREPO) 2>&1 | tee $(ARTIFACTS)/.digest
	@cosign sign --yes $(COSIGN_ARGS) $(HELMREPO)/image-factory@$$(cat $(ARTIFACTS)/.digest | awk -F "[, ]+" '/Digest/{print $$NF}')

.PHONY: chart-lint
chart-lint:  ## Lint helm chart
	@helm lint deploy/helm/image-factory

.PHONY: helm-plugin-install
helm-plugin-install:  ## Install helm plugins
	-helm plugin install https://github.com/helm-unittest/helm-unittest.git --verify=false --version=v1.0.3
	-helm plugin install https://github.com/losisin/helm-values-schema-json.git --verify=false --version=v2.3.1

.PHONY: kuttl-plugin-install
kuttl-plugin-install:  ## Install kubectl kuttl plugin
	kubectl krew install kuttl

.PHONY: chart-e2e
chart-e2e:  ## Run helm chart e2e tests
	export KUBECONFIG=$(shell pwd)/$(ARTIFACTS)/kubeconfig && cd deploy/helm/e2e && kubectl kuttl test

.PHONY: chart-unittest
chart-unittest: $(ARTIFACTS)  ## Run helm chart unit tests
	@helm unittest deploy/helm/image-factory --output-type junit --output-file $(ARTIFACTS)/helm-unittest-report.xml

.PHONY: chart-gen-schema
chart-gen-schema:  ## Generate helm chart schema
	@helm schema --use-helm-docs --draft=7 --indent=2 --values=deploy/helm/image-factory/values.yaml --output=deploy/helm/image-factory/values.schema.json

.PHONY: helm-docs
helm-docs:  ## Runs helm-docs and generates chart documentation
	@$(MAKE) local-$@ DEST=.

.PHONY: imager-base
imager-base:

.PHONY: imager-tools
imager-tools:

.PHONY: integration.test
integration.test:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: integration.enterprise.test
integration.enterprise.test:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: update-to-talos-main
update-to-talos-main:
	@$(MAKE) local-copy-out-go-mod DEST=. TARGET_ARGS="--no-cache-filter=update-to-talos-main"

.PHONY: integration-cdn-talos-main
integration-cdn-talos-main: update-to-talos-main
	@$(MAKE) integration-cdn

.PHONY: integration-direct-talos-main
integration-direct-talos-main: update-to-talos-main
	@$(MAKE) integration-direct

.PHONY: integration-s3-talos-main
integration-s3-talos-main: update-to-talos-main
	@$(MAKE) integration-s3

.PHONY: tailwind
tailwind:
	@$(MAKE) local-tailwind-copy PUSH=false DEST=. PLATFORM=$(OPERATING_SYSTEM)/$(GOARCH)

.PHONY: docs
docs:
	@$(MAKE) local-$@ DEST=docs

.PHONY: check-dirty-ci
check-dirty-ci: check-dirty

.PHONY: docker-compose-up
docker-compose-up:
	@$(MAKE) image-image-factory PUSH=true
	@IMAGE_FACTORY_IMAGE=$(REGISTRY)/$(USERNAME)/image-factory:$(IMAGE_TAG) docker compose -f hack/dev/compose.yaml up --pull=always --remove-orphans --no-attach registry.local --no-attach registry.ghcr.io

.PHONY: docker-compose-down
docker-compose-down:
	@IMAGE_FACTORY_IMAGE=$(REGISTRY)/$(USERNAME)/image-factory:$(IMAGE_TAG) docker compose -f hack/dev/compose.yaml down

.PHONY: talosctl
talosctl: $(ARTIFACTS)
	curl -Lo $(ARTIFACTS)/talosctl https://github.com/siderolabs/talos/releases/download/v$(TALOS_VERSION)/talosctl-$(GOOS)-$(GOARCH)
	chmod +x $(ARTIFACTS)/talosctl

.PHONY: tools
tools: talosctl

.PHONY: k8s-up
k8s-up: $(ARTIFACTS)
	$(ARTIFACTS)/talosctl cluster create docker \
	    --name=image-factory-env \
	    --talosconfig-destination=$(ARTIFACTS)/talosconfig \
	    --kubernetes-version=$(K8S_VERSION) \
	    --mtu=1450
	$(ARTIFACTS)/talosctl kubeconfig $(ARTIFACTS)/kubeconfig \
	    --talosconfig=$(ARTIFACTS)/talosconfig \
	    --nodes=10.5.0.2 \
	    --force

.PHONY: k8s-down
k8s-down:
	$(ARTIFACTS)/talosctl cluster destroy \
	    --name=image-factory-env
	rm -f $(ARTIFACTS)/talosconfig $(ARTIFACTS)/kubeconfig

.PHONY: chart-e2e-ci
chart-e2e-ci: tools
	@$(MAKE) image-image-factory PUSH=true
	@$(MAKE) chart-version CHART_VERSION=v1.0.0-test.1 REGISTRY=$(REGISTRY) USERNAME=$(USERNAME) TAG=$(TAG)
	@$(MAKE) k8s-up
	@$(MAKE) kuttl-plugin-install
	@$(MAKE) chart-e2e

.PHONY: chart-version
chart-version:
	yq -i '.version = strenv(CHART_VERSION)' deploy/helm/image-factory/Chart.yaml
	yq -i '.appVersion = strenv(TAG)' deploy/helm/image-factory/Chart.yaml
	sed -i '/# -- Repository to use for Image Factory/{n; s|repository:.*|repository: '"$(REGISTRY)/$(USERNAME)/image-factory"'|}' deploy/helm/image-factory/values.yaml

.PHONY: rekres
rekres:
	@docker pull $(KRES_IMAGE)
	@docker run --rm --net=host --user $(shell id -u):$(shell id -g) -v $(PWD):/src -w /src -e GITHUB_TOKEN $(KRES_IMAGE)

.PHONY: help
help:  ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: release-notes
release-notes: $(ARTIFACTS)
	@ARTIFACTS=$(ARTIFACTS) ./hack/release.sh $@ $(ARTIFACTS)/RELEASE_NOTES.md $(TAG)

.PHONY: conformance
conformance:
	@docker pull $(CONFORMANCE_IMAGE)
	@docker run --rm -it -v $(PWD):/src -w /src $(CONFORMANCE_IMAGE) enforce

