# syntax = docker/dockerfile-upstream:1.8.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2024-06-30T11:34:44Z by kres 4c9f215.

ARG TOOLCHAIN

FROM alpine:3.18 AS base-image-image-factory

# runs markdownlint
FROM docker.io/oven/bun:1.1.13-alpine AS lint-markdown
WORKDIR /src
RUN bun i markdownlint-cli@0.41.0 sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./CHANGELOG.md ./CHANGELOG.md
COPY ./README.md ./README.md
RUN bunx markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# Installs tailwindcss
FROM docker.io/node:21.7.3-alpine3.19 AS tailwind-base
WORKDIR /src
COPY package.json package-lock.json .
RUN --mount=type=cache,target=/src/node_modules npm ci

# base toolchain image
FROM --platform=${BUILDPLATFORM} ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# tailwind update
FROM tailwind-base AS tailwind-update
COPY tailwind.config.js .
COPY internal/frontend/http internal/frontend/http
RUN --mount=type=cache,target=/src/node_modules node_modules/.bin/tailwindcss -i internal/frontend/http/css/input.css -o internal/frontend/http/css/output.css --minify

# build tools
FROM --platform=${BUILDPLATFORM} toolchain AS tools
ENV GO111MODULE=on
ARG CGO_ENABLED
ENV CGO_ENABLED=${CGO_ENABLED}
ARG GOTOOLCHAIN
ENV GOTOOLCHAIN=${GOTOOLCHAIN}
ARG GOEXPERIMENT
ENV GOEXPERIMENT=${GOEXPERIMENT}
ENV GOPATH=/go
ARG DEEPCOPY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/vuln/cmd/govulncheck@latest \
	&& mv /go/bin/govulncheck /bin/govulncheck
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumpt /bin/gofumpt

# Copies assets
FROM scratch AS tailwind-copy
COPY --from=tailwind-update /src/internal/frontend/http/css/output.css internal/frontend/http/css/output.css

# tools and sources
FROM tools AS base
WORKDIR /src
COPY go.mod go.mod
COPY go.sum go.sum
RUN cd .
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

FROM tools AS embed-generate
ARG SHA
ARG TAG
WORKDIR /src
RUN mkdir -p internal/version/data && \
    echo -n ${SHA} > internal/version/data/sha && \
    echo -n ${TAG} > internal/version/data/tag

# builds the integration test binary
FROM base AS integration-build
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go test -c -covermode=atomic -coverpkg=./... -tags integration ./internal/integration

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
WORKDIR /src
COPY .golangci.yml .
ENV GOGC=50
RUN golangci-lint config verify --config .golangci.yml
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# runs govulncheck
FROM base AS lint-govulncheck
WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg govulncheck ./...

# runs unit-tests with race detector
FROM base AS unit-tests-race
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

FROM embed-generate AS embed-abbrev-generate
WORKDIR /src
ARG ABBREV_TAG
RUN echo -n 'undefined' > internal/version/data/sha && \
    echo -n ${ABBREV_TAG} > internal/version/data/tag

# copies out the integration test binary
FROM scratch AS integration.test
COPY --from=integration-build /src/integration.test /integration.test

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage-unit-tests.txt

# cleaned up specs and compiled versions
FROM scratch AS generate
COPY --from=embed-abbrev-generate /src/internal/version internal/version

# builds image-factory-linux-amd64
FROM base AS image-factory-linux-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/image-factory
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=image-factory -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /image-factory-linux-amd64

# builds image-factory-linux-arm64
FROM base AS image-factory-linux-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/image-factory
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG VERSION_PKG="internal/version"
ARG SHA
ARG TAG
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=image-factory -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /image-factory-linux-arm64

FROM scratch AS image-factory-linux-amd64
COPY --from=image-factory-linux-amd64-build /image-factory-linux-amd64 /image-factory-linux-amd64

FROM scratch AS image-factory-linux-arm64
COPY --from=image-factory-linux-arm64-build /image-factory-linux-arm64 /image-factory-linux-arm64

FROM image-factory-linux-${TARGETARCH} AS image-factory

FROM scratch AS image-factory-all
COPY --from=image-factory-linux-amd64 / /
COPY --from=image-factory-linux-arm64 / /

FROM base-image-image-factory AS image-image-factory
RUN apk add --no-cache --update bash binutils-aarch64 binutils-x86_64 cpio dosfstools efibootmgr kmod mtools pigz qemu-img squashfs-tools tar util-linux xfsprogs xorriso xz zstd
ARG TARGETARCH
COPY --from=image-factory image-factory-linux-${TARGETARCH} /image-factory
COPY --from=ghcr.io/siderolabs/grub:v1.7.0-2-g6101299 / /
COPY --from=ghcr.io/siderolabs/grub@sha256:46469ae913378d45f69ac10d2dc8ebea54e914542deab2b2f23c95dac5116335 /usr/lib/grub /usr/lib/grub
COPY --from=ghcr.io/siderolabs/grub@sha256:fd929bae5ad64a3e2a530d6b3cbfab673b60af2e62268a45cf42194df55c116d /usr/lib/grub /usr/lib/grub
COPY --from=ghcr.io/siderolabs/installer:v1.7.0 /usr/share/grub/unicode.pf2 /usr/share/grub/unicode.pf2
LABEL org.opencontainers.image.source=https://github.com/siderolabs/image-factory
ENTRYPOINT ["/image-factory"]

