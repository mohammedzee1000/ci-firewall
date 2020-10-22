#!/usr/bin/env bash
rm -rf dist
mkdir -p dist/build/linux/amd64
go build -mod=vendor -o dist/build/linux/amd64/ci-firewall cmd/ci-firewall/ci-firewall.go
go build -mod=vendor -o dist/build/linux/amd64/simple-ssh-execute cmd/simple-ssh-execute/simple-ssh-execute.go
