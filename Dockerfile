# syntax = docker/dockerfile-upstream:1.2.0-labs

# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2022-06-28T16:24:58Z by kres ba905fc-dirty.

ARG TOOLCHAIN

FROM ghcr.io/siderolabs/ca-certificates:v1.0.0 AS image-ca-certificates

FROM ghcr.io/siderolabs/fhs:v1.0.0 AS image-fhs

# runs markdownlint
FROM node:18.2.0-alpine AS lint-markdown
WORKDIR /src
RUN npm i -g markdownlint-cli@0.31.1
RUN npm i sentences-per-line@0.2.1
COPY .markdownlint.json .
COPY ./README.md ./README.md
RUN markdownlint --ignore "CHANGELOG.md" --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' --rules node_modules/sentences-per-line/index.js .

# collects proto specs
FROM scratch AS proto-specs
ADD https://raw.githubusercontent.com/cosi-project/specification/fd31e32e8060fec72a4f9ee7f3b7d1924ea3e4d0/proto/v1alpha1/resource.proto /api/v1alpha1/
ADD https://raw.githubusercontent.com/cosi-project/specification/fd31e32e8060fec72a4f9ee7f3b7d1924ea3e4d0/proto/v1alpha1/state.proto /api/v1alpha1/
ADD https://raw.githubusercontent.com/cosi-project/specification/fd31e32e8060fec72a4f9ee7f3b7d1924ea3e4d0/proto/v1alpha1/runtime.proto /api/v1alpha1/
ADD https://raw.githubusercontent.com/cosi-project/specification/fd31e32e8060fec72a4f9ee7f3b7d1924ea3e4d0/proto/v1alpha1/meta.proto /api/v1alpha1/

# base toolchain image
FROM ${TOOLCHAIN} AS toolchain
RUN apk --update --no-cache add bash curl build-base protoc protobuf-dev

# build tools
FROM toolchain AS tools
ENV GO111MODULE on
ENV CGO_ENABLED 0
ENV GOPATH /go
ARG GOLANGCILINT_VERSION
RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCILINT_VERSION}/install.sh | bash -s -- -b /bin ${GOLANGCILINT_VERSION}
ARG GOFUMPT_VERSION
RUN go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
	&& mv /go/bin/gofumpt /bin/gofumpt
ARG GOIMPORTS_VERSION
RUN go install golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION} \
	&& mv /go/bin/goimports /bin/goimports
ARG PROTOBUF_GO_VERSION
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v${PROTOBUF_GO_VERSION}
RUN mv /go/bin/protoc-gen-go /bin
ARG GRPC_GO_VERSION
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${GRPC_GO_VERSION}
RUN mv /go/bin/protoc-gen-go-grpc /bin
ARG GRPC_GATEWAY_VERSION
RUN go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v${GRPC_GATEWAY_VERSION}
RUN mv /go/bin/protoc-gen-grpc-gateway /bin
ARG VTPROTOBUF_VERSION
RUN go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@v${VTPROTOBUF_VERSION}
RUN mv /go/bin/protoc-gen-go-vtproto /bin
ARG DEEPCOPY_VERSION
RUN go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
	&& mv /go/bin/deep-copy /bin/deep-copy

# tools and sources
FROM tools AS base
WORKDIR /src
COPY ./go.mod .
COPY ./go.sum .
RUN --mount=type=cache,target=/go/pkg go mod download
RUN --mount=type=cache,target=/go/pkg go mod verify
COPY ./api ./api
COPY ./cmd ./cmd
COPY ./pkg ./pkg
RUN --mount=type=cache,target=/go/pkg go list -mod=readonly all >/dev/null

# runs protobuf compiler
FROM tools AS proto-compile
COPY --from=proto-specs / /
RUN protoc -I/api --grpc-gateway_out=paths=source_relative:/api --grpc-gateway_opt=generate_unbound_methods=true --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size --experimental_allow_proto3_optional /api/v1alpha1/resource.proto /api/v1alpha1/state.proto /api/v1alpha1/runtime.proto /api/v1alpha1/meta.proto
RUN rm /api/v1alpha1/resource.proto
RUN rm /api/v1alpha1/state.proto
RUN rm /api/v1alpha1/runtime.proto
RUN rm /api/v1alpha1/meta.proto
RUN goimports -w -local github.com/cosi-project/runtime /api
RUN gofumpt -w /api

# runs gofumpt
FROM base AS lint-gofumpt
RUN FILES="$(gofumpt -l .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumpt -w .':\n${FILES}"; exit 1)

# runs goimports
FROM base AS lint-goimports
RUN FILES="$(goimports -l -local github.com/cosi-project/runtime .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'goimports -w -local github.com/cosi-project/runtime .':\n${FILES}"; exit 1)

# runs golangci-lint
FROM base AS lint-golangci-lint
COPY .golangci.yml .
ENV GOGC 50
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.cache/golangci-lint --mount=type=cache,target=/go/pkg golangci-lint run --config .golangci.yml

# runs unit-tests with race detector
FROM base AS unit-tests-race
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp CGO_ENABLED=1 go test -v -race -count 1 ${TESTPKGS}

# runs unit-tests
FROM base AS unit-tests-run
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg --mount=type=cache,target=/tmp go test -v -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 ${TESTPKGS}

# cleaned up specs and compiled versions
FROM scratch AS generate
COPY --from=proto-compile /api/ /api/

FROM scratch AS unit-tests
COPY --from=unit-tests-run /src/coverage.txt /coverage.txt

# builds runtime-linux-amd64
FROM base AS runtime-linux-amd64-build
COPY --from=generate / /
WORKDIR /src/cmd/runtime
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/go/pkg go build -ldflags "-s -w" -o /runtime-linux-amd64

FROM scratch AS runtime-linux-amd64
COPY --from=runtime-linux-amd64-build /runtime-linux-amd64 /runtime-linux-amd64

FROM runtime-linux-${TARGETARCH} AS runtime

FROM scratch AS image-runtime
ARG TARGETARCH
COPY --from=runtime runtime-linux-${TARGETARCH} /runtime
COPY --from=image-fhs / /
COPY --from=image-ca-certificates / /
LABEL org.opencontainers.image.source https://github.com/cosi-project/runtime
ENTRYPOINT ["/runtime"]

