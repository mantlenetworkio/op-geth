# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build Geth in a stock Go builder container
FROM golang:1.24-alpine AS builder

ARG COMMIT=""
ARG VERSION=""

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum
RUN cd /go-ethereum && \
	GIT_TAG="$(git tag -l --points-at HEAD | head -n 1)" && \
	set -- go run build/ci.go install -static && \
	if [ -n "$VERSION" ]; then set -- "$@" -git-tag "$VERSION"; fi && \
	if [ -n "$COMMIT" ]; then set -- "$@" -git-commit "$COMMIT"; fi && \
	if [ -z "$VERSION" ] && [ -n "$GIT_TAG" ]; then set -- "$@" -git-tag "$GIT_TAG"; fi && \
	set -- "$@" ./cmd/geth && \
	"$@"

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates  jq
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/
COPY entrypoint.sh /app/entrypoint.sh

EXPOSE 8545 8546 30303 30303/udp

WORKDIR /app

ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]

# Add some metadata labels to help programmatic image consumption
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
