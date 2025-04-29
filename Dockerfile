# syntax = docker/dockerfile-upstream:1.14.1-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2025-04-17T10:39:42Z by kres fd5cab0.

ARG TOOLCHAIN
ARG PKGS_PREFIX
ARG PKGS

# runs markdownlint
FROM docker.io/oven/bun:1.2.9-alpine AS lint-markdown
WORKDIR /src
RUN bun i markdownlint-cli@0.44.0 sentences-per-line@0.3.0
COPY .markdownlint.json .
COPY ./CHANGELOG.md ./CHANGELOG.md
COPY ./README.md ./README.md
RUN bunx markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules sentences-per-line .

FROM ${PKGS_PREFIX}/ca-certificates:${PKGS} AS pkg-ca-certificates

FROM ${PKGS_PREFIX}/cpio:${PKGS} AS pkg-cpio

FROM ${PKGS_PREFIX}/dosfstools:${PKGS} AS pkg-dosfstools

FROM ${PKGS_PREFIX}/e2fsprogs:${PKGS} AS pkg-e2fsprogs

FROM ${PKGS_PREFIX}/fhs:${PKGS} AS pkg-fhs

FROM ${PKGS_PREFIX}/glib:${PKGS} AS pkg-glib

FROM ${PKGS_PREFIX}/grub:${PKGS} AS pkg-grub

FROM --platform=linux/amd64 ${PKGS_PREFIX}/grub:${PKGS} AS pkg-grub-amd64

FROM --platform=linux/arm64 ${PKGS_PREFIX}/grub:${PKGS} AS pkg-grub-arm64

FROM ${PKGS_PREFIX}/installer:v1.9.4 AS pkg-grub-unicode

FROM ${PKGS_PREFIX}/kmod:${PKGS} AS pkg-kmod

FROM ${PKGS_PREFIX}/libattr:${PKGS} AS pkg-libattr

FROM ${PKGS_PREFIX}/libburn:${PKGS} AS pkg-libburn

FROM ${PKGS_PREFIX}/libinih:${PKGS} AS pkg-libinih

FROM ${PKGS_PREFIX}/libisoburn:${PKGS} AS pkg-libisoburn

FROM ${PKGS_PREFIX}/libisofs:${PKGS} AS pkg-libisofs

FROM ${PKGS_PREFIX}/liblzma:${PKGS} AS pkg-liblzma

FROM ${PKGS_PREFIX}/liburcu:${PKGS} AS pkg-liburcu

FROM ${PKGS_PREFIX}/mtools:${PKGS} AS pkg-mtools

FROM ${PKGS_PREFIX}/musl:${PKGS} AS pkg-musl

FROM ${PKGS_PREFIX}/openssl:${PKGS} AS pkg-openssl

FROM ${PKGS_PREFIX}/pcre2:${PKGS} AS pkg-pcre2

FROM ${PKGS_PREFIX}/pigz:${PKGS} AS pkg-pigz

FROM ${PKGS_PREFIX}/qemu-tools:${PKGS} AS pkg-qemu-tools

FROM ${PKGS_PREFIX}/squashfs-tools:${PKGS} AS pkg-squashfs-tools

FROM ${PKGS_PREFIX}/tar:${PKGS} AS pkg-tar

FROM ${PKGS_PREFIX}/xfsprogs:${PKGS} AS pkg-xfsprogs

FROM ${PKGS_PREFIX}/xz:${PKGS} AS pkg-xz

FROM ${PKGS_PREFIX}/zlib:${PKGS} AS pkg-zlib

FROM ${PKGS_PREFIX}/zstd:${PKGS} AS pkg-zstd

# Installs tailwindcss
FROM --platform=${BUILDPLATFORM} docker.io/oven/bun:1.2.4-alpine AS tailwind-base
WORKDIR /src
COPY package.json package-lock.json .
RUN --mount=type=cache,target=/src/node_modules,id=image-factory/src/node_modules bun install

# base toolchain image
FROM --platform=${BUILDPLATFORM} ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# copies the imager tools
FROM scratch AS imager-tools
COPY --from=pkg-fhs / /
COPY --from=pkg-ca-certificates / /
COPY --from=pkg-musl / /
COPY --from=pkg-cpio / /
COPY --from=pkg-dosfstools / /
COPY --from=pkg-grub / /
COPY --from=pkg-grub-amd64 /usr/lib/grub /usr/lib/grub
COPY --from=pkg-grub-arm64 /usr/lib/grub /usr/lib/grub
COPY --from=pkg-grub-unicode /usr/share/grub/unicode.pf2 /usr/share/grub/unicode.pf2
COPY --from=pkg-kmod / /
COPY --from=pkg-libattr / /
COPY --from=pkg-libinih / /
COPY --from=pkg-liblzma / /
COPY --from=pkg-liburcu / /
COPY --from=pkg-openssl / /
COPY --from=pkg-xfsprogs / /
COPY --from=pkg-e2fsprogs / /
COPY --from=pkg-glib / /
COPY --from=pkg-libburn / /
COPY --from=pkg-libisoburn / /
COPY --from=pkg-libisofs / /
COPY --from=pkg-mtools / /
COPY --from=pkg-pcre2 / /
COPY --from=pkg-pigz / /
COPY --from=pkg-qemu-tools / /
COPY --from=pkg-squashfs-tools / /
COPY --from=pkg-tar / /
COPY --from=pkg-xz / /
COPY --from=pkg-zlib / /
COPY --from=pkg-zstd / /

# tailwind update
FROM tailwind-base AS tailwind-update
COPY tailwind.config.js .
COPY internal/frontend/http internal/frontend/http
RUN --mount=type=cache,target=/src/node_modules,id=image-factory/src/node_modules node_modules/.bin/tailwindcss -i internal/frontend/http/css/input.css -o internal/frontend/http/css/output.css --minify

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
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go install golang.org/x/vuln/cmd/govulncheck@latest \
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
RUN --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go mod verify
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go list -mod=readonly all >/dev/null

FROM tools AS embed-generate
ARG SHA
ARG TAG
WORKDIR /src
RUN mkdir -p internal/version/data && \
    echo -n ${SHA} > internal/version/data/sha && \
    echo -n ${TAG} > internal/version/data/tag

# builds the integration test binary
FROM base AS integration-build
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go test -c -covermode=atomic -coverpkg=./... -tags integration ./internal/integration

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
WORKDIR /src
COPY .golangci.yml .
ENV GOGC=50
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint,id=image-factory/root/.cache/golangci-lint,sharing=locked --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg golangci-lint run --config .golangci.yml

# runs govulncheck
FROM base AS lint-govulncheck
WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg govulncheck ./...

# runs unit-tests with race detector
FROM base AS unit-tests-race
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg --mount=type=cache,target=/tmp,id=image-factory/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg --mount=type=cache,target=/tmp,id=image-factory/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

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
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=image-factory -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /image-factory-linux-amd64

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
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} -X ${VERSION_PKG}.Name=image-factory -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /image-factory-linux-arm64

FROM scratch AS image-factory-linux-amd64
COPY --from=image-factory-linux-amd64-build /image-factory-linux-amd64 /image-factory-linux-amd64

FROM scratch AS image-factory-linux-arm64
COPY --from=image-factory-linux-arm64-build /image-factory-linux-arm64 /image-factory-linux-arm64

FROM image-factory-linux-${TARGETARCH} AS image-factory

FROM scratch AS image-factory-all
COPY --from=image-factory-linux-amd64 / /
COPY --from=image-factory-linux-arm64 / /

FROM scratch AS image-image-factory
ARG TARGETARCH
COPY --from=image-factory image-factory-linux-${TARGETARCH} /usr/bin/image-factory
COPY --from=imager-tools / /
LABEL org.opencontainers.image.source=https://github.com/siderolabs/image-factory
ENTRYPOINT ["/usr/bin/image-factory"]

