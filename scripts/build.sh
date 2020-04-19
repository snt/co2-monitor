#!/usr/bin/env bash

mkdir -p $(dirname $0)/../dist/

pushd $(dirname $0)/../cmd/co2-monitor
GOOS=linux GOARCH=arm GOARM=7 go build
mv co2-monitor ../../dist
popd

pushd $(dirname $0)/../cmd/co2-spreadsheet-recorder
GOOS=linux GOARCH=arm GOARM=7 go build
mv co2-spreadsheet-recorder ../../dist
popd
