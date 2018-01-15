#!/bin/bash

mkdir dist
GOOS=linux GOARCH=386 CGO_ENABLED=0 go build -o agent.linux .
tar czf dist/nginxagent.tar.gz ./agent.linux
rm -f ./agent.linux