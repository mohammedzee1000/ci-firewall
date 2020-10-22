#!/usr/bin/env bash
go build -mod=vendor -o dist/ci-firewall cmd/ci-firewall/ci-firewall.go
go build -mod=vendor -o dist/ssh-run-cmd cmd/ssh-run-cmd/ssh-run-cmd.go
