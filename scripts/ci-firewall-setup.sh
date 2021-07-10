#!/usr/bin/bash
PROJECT=github.com/mohammedzee1000/ci-firewall
GITCOMMIT=${GITCOMMIT:-$(git rev-parse --short HEAD 2>/dev/null)}
VERSION=${VERSION:="master"}
go build -ldflags="-X ${PROJECT}/pkg/version.GITCOMMIT=${GITCOMMIT} -X ${PROJECT}/pkg/version.VERSION=${VERSION}" -mod=vendor -o /usr/local/bin/ci-firewall cmd/ci-firewall/ci-firewall.go
ls
ls /usr/local/bin/