#!/bin/bash

set -e

migrate -path=db/migrations -database "postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=${POSTGRES_SSLMODE}&channel_binding=${POSTGRES_CHANNEL_BINDING}" -verbose ${1}
