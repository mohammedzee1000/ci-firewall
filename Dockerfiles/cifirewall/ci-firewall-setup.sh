#!/usr/bin/bash

#download ci-firewall
curl -kJLO https://github.com/mohammedzee1000/ci-firewall/releases/download/${CI_FIREWALL_VERSION}/ci-firewall-linux-amd64.tar.gz
tar -xzf ./ci-firewall-linux-amd64.tar.gz && rm -rf ./ci-firewall-linux-amd64.tar.gz && chmod +x ./ci-firewall
