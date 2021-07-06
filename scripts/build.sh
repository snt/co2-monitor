#!/usr/bin/env bash

mkdir -p $(dirname $0)/../dist/

cd $(dirname $0)/..
GOOS=linux GOARCH=arm GOARM=7 go build
mv co2-monitor dist
