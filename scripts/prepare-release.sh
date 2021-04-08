#!/usr/bin/env bash

export VERSION="v0.1.0"
sh scripts/build.sh
rm -rf dist/release
mkdir dist/release
pushd dist/build/linux/amd64
ls ../../../release
tar -czf ci-firewall-linux-amd64.tar.gz ci-firewall
#tar -czf simple-ssh-execute-linux-amd64.tar.gz simple-ssh-execute
mv *.tar.gz ../../../release/
popd
