#!/usr/bin/env bash
PROJECT=github.com/mohammedzee1000/ci-firewall
GITCOMMIT=${GITCOMMIT:-$(git rev-parse --short HEAD 2>/dev/null)}
VERSION=${VERSION:="master"}
rm -rf dist
mkdir -p dist/build/linux/amd64
CGO_ENABLED=0 go build -ldflags="-extldflags=-static -X ${PROJECT}/pkg/version.GITCOMMIT=${GITCOMMIT} -X ${PROJECT}/pkg/version.VERSION=${VERSION}" -mod=vendor -o dist/build/linux/amd64/ci-firewall cmd/ci-firewall/ci-firewall.go
