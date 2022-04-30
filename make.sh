#!/bin/sh

[ ! -d bin ] && mkdir bin

set -x
go build -o bin/release
