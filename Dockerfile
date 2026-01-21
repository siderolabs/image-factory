# syntax = docker/dockerfile-upstream:1.20.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2026-01-21T13:19:04Z by kres 1ffefb6.

ARG TOOLCHAIN=scratch
ARG PKGS_PREFIX=scratch
ARG PKGS=scratch

# runs markdownlint
FROM docker.io/oven/bun:1.3.6-alpine AS lint-markdown
WORKDIR /src
RUN bun i markdownlint-cli@0.47.0 sentences-per-line@0.5.0
COPY .markdownlint.json .
COPY ./docs ./docs
COPY ./CHANGELOG.md ./CHANGELOG.md
COPY ./README.md ./README.md
RUN bunx markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules markdownlint-sentences-per-line .

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

FROM ${PKGS_PREFIX}/libarchive:${PKGS} AS pkg-libarchive

FROM ${PKGS_PREFIX}/libattr:${PKGS} AS pkg-libattr

FROM ${PKGS_PREFIX}/libburn:${PKGS} AS pkg-libburn

FROM ${PKGS_PREFIX}/libinih:${PKGS} AS pkg-libinih

FROM ${PKGS_PREFIX}/libisoburn:${PKGS} AS pkg-libisoburn

FROM ${PKGS_PREFIX}/libisofs:${PKGS} AS pkg-libisofs

FROM ${PKGS_PREFIX}/liblzma:${PKGS} AS pkg-liblzma

FROM ${PKGS_PREFIX}/liburcu:${PKGS} AS pkg-liburcu

FROM ${PKGS_PREFIX}/mtools:${PKGS} AS pkg-mtools

FROM ${PKGS_PREFIX}/musl:${PKGS} AS pkg-musl

FROM ${PKGS_PREFIX}/open-vmdk:${PKGS} AS pkg-open-vmdk

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
RUN bun install

# base toolchain image
FROM --platform=${BUILDPLATFORM} ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash build-base curl jq protoc protobuf-dev

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
COPY --from=pkg-libarchive / /
COPY --from=pkg-libattr / /
COPY --from=pkg-libinih / /
COPY --from=pkg-liblzma / /
COPY --from=pkg-liburcu / /
COPY --from=pkg-openssl / /
COPY --from=pkg-open-vmdk / /
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
RUN node_modules/.bin/tailwindcss -i internal/frontend/http/css/input.css -o internal/frontend/http/css/output.css --minify

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
COPY ./enterprise ./enterprise
COPY ./tools ./tools
RUN --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go list -mod=readonly all >/dev/null

FROM tools AS embed-generate
ARG SHA
ARG TAG
WORKDIR /src
RUN mkdir -p internal/version/data && \
    echo -n ${SHA} > internal/version/data/sha && \
    echo -n ${TAG} > internal/version/data/tag

# run the docgen
FROM base AS docgen
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go run ./tools/docgen ./cmd/image-factory/cmd/options.go docs/configuration.md

# builds the integration test binary
FROM base AS integration-build
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go test -c -covermode=atomic -coverpkg=./... -tags integration ./internal/integration

# builds the integration test binary (Enterprise flavor)
FROM base AS integration-enterprise-build
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go test -c -covermode=atomic -coverpkg=./... -tags integration,enterprise ./internal/integration

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
WORKDIR /src
COPY .golangci.yml .
ENV GOGC=50
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint,id=image-factory/root/.cache/golangci-lint,sharing=locked --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg golangci-lint run --config .golangci.yml

# runs golangci-lint fmt
FROM base AS lint-golangci-lint-fmt-run
WORKDIR /src
COPY .golangci.yml .
ENV GOGC=50
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint,id=image-factory/root/.cache/golangci-lint,sharing=locked --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg golangci-lint fmt --config .golangci.yml
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint,id=image-factory/root/.cache/golangci-lint,sharing=locked --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg golangci-lint run --fix --issues-exit-code 0 --config .golangci.yml

# runs govulncheck
FROM base AS lint-govulncheck
WORKDIR /src
COPY --chmod=0755 hack/govulncheck.sh ./hack/govulncheck.sh
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg ./hack/govulncheck.sh ./...

# runs unit-tests with race detector
FROM base AS unit-tests-race
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg --mount=type=cache,target=/tmp,id=image-factory/tmp CGO_ENABLED=1 go test -race ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg --mount=type=cache,target=/tmp,id=image-factory/tmp go test -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} ${TESTPKGS}

# updates go.mod to use the latest talos main
FROM base AS update-to-talos-main
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go get -u github.com/siderolabs/talos@main
RUN --mount=type=cache,target=/root/.cache/go-build,id=image-factory/root/.cache/go-build --mount=type=cache,target=/go/pkg,id=image-factory/go/pkg go get -u github.com/siderolabs/talos/pkg/machinery@main

FROM embed-generate AS embed-abbrev-generate
WORKDIR /src
ARG ABBREV_TAG
RUN echo -n 'undefined' > internal/version/data/sha && \
    echo -n ${ABBREV_TAG} > internal/version/data/tag

# copies out the generated docs
FROM scratch AS docs
COPY --from=docgen /src/docs/configuration.md /configuration.md

# copies out the integration test binary
FROM scratch AS integration.test
COPY --from=integration-build /src/integration.test /integration.test

# copies out the integration test binary
FROM scratch AS integration.enterprise.test
COPY --from=integration-enterprise-build /src/integration.test /integration.enterprise.test

# clean golangci-lint fmt output
FROM scratch AS lint-golangci-lint-fmt
COPY --from=lint-golangci-lint-fmt-run /src .

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage-unit-tests.txt

# copies out the go.mod and go.sum
FROM scratch AS copy-out-go-mod
COPY --from=update-to-talos-main /src/go.mod /go.mod
COPY --from=update-to-talos-main /src/go.sum /go.sum

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
ENV TUF_ROOT=/tmp
ENTRYPOINT ["/usr/bin/image-factory"]

