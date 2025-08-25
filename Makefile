include .env
export $(shell sed 's/=.*//' .env)

cli: build
	set -a
	. ./.env
	set +a
	./bin/cli $(args)

db-migration-create:
	./db/create-migration.sh $(name)

db-migration-apply:
	./db/apply-migration.sh $(op) $(sslmode)

build:
	go build -o bin/api ./cmd/api
	chmod +x bin/api

	go build -o bin/cli ./cmd/cli
	chmod +x bin/cli

run-api:
	air --build.cmd "make build" --build.bin "./bin/api"
