#!/usr/bin/env bash
go build -mod=vendor -o dist/ci-firewall cmd/ci-firewall/ci-firewall.go
go build -mod=vendor -o dist/simple-ssh-execute cmd/simple-ssh-execute/simple-ssh-execute.go
