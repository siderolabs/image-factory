# syntax = docker/dockerfile-upstream:1.6.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2023-08-31T14:21:15Z by kres latest.

ARG TOOLCHAIN

FROM alpine:3.18 AS base-image-image-service

# runs markdownlint
FROM docker.io/node:20.5.1-alpine3.18 AS lint-markdown
WORKDIR /src
RUN npm i -g markdownlint-cli@0.35.0
RUN npm i sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./README.md ./README.md
RUN markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# base toolchain image
FROM ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# build tools
FROM --platform=${BUILDPLATFORM} toolchain AS tools
ENV GO111MODULE on
ARG CGO_ENABLED
ENV CGO_ENABLED ${CGO_ENABLED}
ARG GOTOOLCHAIN
ENV GOTOOLCHAIN ${GOTOOLCHAIN}
ARG GOEXPERIMENT
ENV GOEXPERIMENT ${GOEXPERIMENT}
ENV GOPATH /go
ARG DEEPCOPY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /bin/golangci-lint
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/vuln/cmd/govulncheck@latest \
	&& mv /go/bin/govulncheck /bin/govulncheck
ARG GOIMPORTS_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION} \
	&& mv /go/bin/goimports /bin/goimports
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumpt /bin/gofumpt

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
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go test -c  -covermode=atomic -coverpkg=./... -tags integration ./internal/integration

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs goimports
FROM base AS lint-goimports
RUN FILES="$(goimports -l -local github.com/siderolabs/image-service/ .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'goimports -w -local github.com/siderolabs/image-service/ .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
WORKDIR /src
COPY .golangci.yml .
ENV GOGC 50
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

# builds image-service-linux-amd64
FROM base AS image-service-linux-amd64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/image-service
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=amd64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /image-service-linux-amd64

# builds image-service-linux-arm64
FROM base AS image-service-linux-arm64-build
COPY --from=generate / /
COPY --from=embed-generate / /
WORKDIR /src/cmd/image-service
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg GOARCH=arm64 GOOS=linux go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /image-service-linux-arm64

FROM scratch AS image-service-linux-amd64
COPY --from=image-service-linux-amd64-build /image-service-linux-amd64 /image-service-linux-amd64

FROM scratch AS image-service-linux-arm64
COPY --from=image-service-linux-arm64-build /image-service-linux-arm64 /image-service-linux-arm64

FROM image-service-linux-${TARGETARCH} AS image-service

FROM scratch AS image-service-all
COPY --from=image-service-linux-amd64 / /
COPY --from=image-service-linux-arm64 / /

FROM base-image-image-service AS image-image-service
RUN apk add --no-cache --update bash binutils-aarch64 binutils-x86_64 cpio dosfstools efibootmgr kmod mtools pigz qemu-img squashfs-tools tar util-linux xfsprogs xorriso xz
ARG TARGETARCH
COPY --from=image-service image-service-linux-${TARGETARCH} /image-service
COPY --from=ghcr.io/siderolabs/grub:v1.5.0 / /
COPY --from=ghcr.io/siderolabs/grub@sha256:b3200d08c4c9295e02dfb11e5760c5dbe2ffe1b1191272851ab03e527220094c /usr/lib/grub /usr/lib/grub
COPY --from=ghcr.io/siderolabs/grub@sha256:a8936963fc5f1fb83b057eb2369371ec7a4e7a9cfda71a804c5d6f8de222abea /usr/lib/grub /usr/lib/grub
COPY --from=ghcr.io/siderolabs/installer:v1.5.0 /usr/share/grub/unicode.pf2 /usr/share/grub/unicode.pf2
LABEL org.opencontainers.image.source https://github.com/siderolabs/image-service
ENTRYPOINT ["/image-service"]

