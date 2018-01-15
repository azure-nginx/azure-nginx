#!/bin/bash

mkdir dist
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o cp.linux .
tar czf dist/controlplane.tar.gz ./cp.linux
rm -f ./cp.linux