# syntax = docker/dockerfile-upstream:1.6.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2024-02-28T13:50:02Z by kres latest.

ARG TOOLCHAIN

FROM ghcr.io/siderolabs/ca-certificates:v1.6.0 AS image-ca-certificates

FROM ghcr.io/siderolabs/fhs:v1.6.0 AS image-fhs

# runs markdownlint
FROM docker.io/node:21.6.2-alpine3.19 AS lint-markdown
WORKDIR /src
RUN npm i -g markdownlint-cli@0.39.0
RUN npm i sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./README.md ./README.md
RUN markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# collects proto specs
FROM scratch AS proto-specs
ADD https://raw.githubusercontent.com/cosi-project/specification/5c734257bfa6a3acb01417809797dbfbe0e73c71/proto/v1alpha1/resource.proto /api/v1alpha1/
ADD https://raw.githubusercontent.com/cosi-project/specification/5c734257bfa6a3acb01417809797dbfbe0e73c71/proto/v1alpha1/state.proto /api/v1alpha1/
ADD https://raw.githubusercontent.com/cosi-project/specification/5c734257bfa6a3acb01417809797dbfbe0e73c71/proto/v1alpha1/meta.proto /api/v1alpha1/
ADD api/key_storage/key_storage.proto /api/key_storage/

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
ARG PROTOBUF_GO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install google.golang.org/protobuf/cmd/protoc-gen-go@v${PROTOBUF_GO_VERSION}
RUN mv /go/bin/protoc-gen-go /bin
ARG GRPC_GO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${GRPC_GO_VERSION}
RUN mv /go/bin/protoc-gen-go-grpc /bin
ARG GRPC_GATEWAY_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v${GRPC_GATEWAY_VERSION}
RUN mv /go/bin/protoc-gen-grpc-gateway /bin
ARG VTPROTOBUF_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@v${VTPROTOBUF_VERSION}
RUN mv /go/bin/protoc-gen-go-vtproto /bin
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
COPY ./api ./api
COPY ./cmd ./cmd
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

# runs protobuf compiler
FROM tools AS proto-compile
COPY --from=proto-specs / /
RUN protoc -I/api --grpc-gateway_out=paths=source_relative:/api --grpc-gateway_opt=generate_unbound_methods=true --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size+equal+clone --experimental_allow_proto3_optional /api/v1alpha1/resource.proto /api/v1alpha1/state.proto /api/v1alpha1/meta.proto
RUN protoc -I/api --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size+equal+clone --experimental_allow_proto3_optional /api/key_storage/key_storage.proto
RUN rm /api/v1alpha1/resource.proto
RUN rm /api/v1alpha1/state.proto
RUN rm /api/v1alpha1/meta.proto
RUN rm /api/key_storage/key_storage.proto
RUN goimports -w -local github.com/cosi-project/runtime /api
RUN gofumpt -w /api

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs goimports
FROM base AS lint-goimports
RUN FILES="$(goimports -l -local github.com/cosi-project/runtime/ .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'goimports -w -local github.com/cosi-project/runtime/ .':\n${FILES}"; exit 1)

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
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 -p 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
WORKDIR /src
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 -p 1 ${TESTPKGS}

# cleaned up specs and compiled versions
FROM scratch AS generate
COPY --from=proto-compile /api/ /api/

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage-unit-tests.txt

# builds runtime-linux-amd64
FROM base AS runtime-linux-amd64-build
COPY --from=generate / /
WORKDIR /src/cmd/runtime
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /runtime-linux-amd64

FROM scratch AS runtime-linux-amd64
COPY --from=runtime-linux-amd64-build /runtime-linux-amd64 /runtime-linux-amd64

FROM runtime-linux-${TARGETARCH} AS runtime

FROM scratch AS runtime-all
COPY --from=runtime-linux-amd64 / /

FROM scratch AS image-runtime
ARG TARGETARCH
COPY --from=runtime runtime-linux-${TARGETARCH} /runtime
COPY --from=image-fhs / /
COPY --from=image-ca-certificates / /
LABEL org.opencontainers.image.source https://github.com/cosi-project/runtime
ENTRYPOINT ["/runtime"]

