#!/bin/bash

set -e

migrate create -ext=sql -dir=db/migrations -seq ${1}
