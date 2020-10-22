#!/usr/bin/env bash

go build -mod=vendor -o dist/ci-firewall cmd/ci-firewall/ci-firewall.go
go build -mod=vendor -o dist/ssh-run-cmd cmd/ssh-run-cmd/ssh-run-cmd.go
pushd dist
tar -czf ci-firewall.tar.gz ci-firewall
tar -czf ssh-run-cmd.tar.gz ssh-run-cmd
popd
