#!/bin/bash

set -e

SCRIPT_DIR=$(dirname ${0})

go build -o ${SCRIPT_DIR}/bin/vo2 *.go
chmod +x ${SCRIPT_DIR}/bin/vo2
