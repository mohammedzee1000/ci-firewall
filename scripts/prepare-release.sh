#!/usr/bin/env bash

sh scripts/build.sh
pushd dist
tar -czf ci-firewall.tar.gz ci-firewall
tar -czf simple-ssh-execute.tar.gz simple-ssh-execute
popd
