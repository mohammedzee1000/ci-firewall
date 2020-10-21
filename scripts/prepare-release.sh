#!/usr/bin/env bash

go build -mod=vendor -o dist/ci-firewall cmd/ci-firewall/ci-firewall.go
tar -czf dist/ci-firewall.tar.gz dist/ci-firewall
