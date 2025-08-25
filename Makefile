include .env
export $(shell sed 's/=.*//' .env)

db-migration-create:
	./db/create-migration.sh $(name)

db-migration-apply:
	./db/apply-migration.sh $(op) $(sslmode)

build:
	go build -o bin/api ./cmd/api
	go build -o bin/cli ./cmd/cli

run-api:
	air --build.cmd "make build" --build.bin "./bin/api"
