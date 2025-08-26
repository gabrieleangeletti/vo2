env ?= dev

ifeq ($(env),docker)
    ENV_FILE = .env.docker

else ifeq ($(env),dev)
    ENV_FILE = .env.dev

else ifeq ($(env),prod)
    ENV_FILE = .env.prod

else
    $(error Invalid environment: $(env). Use env=docker|dev|prod)
endif

ifeq ($(wildcard $(ENV_FILE)),)
    $(error Environment file $(ENV_FILE) does not exist)
endif

include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

cli: build
	set -a
	. $(ENV_FILE)
	set +a
	./bin/cli $(args)

db-migration-create:
	./db/create-migration.sh $(name)

db-migration-apply:
	./db/apply-migration.sh $(op)

build:
	go build -o bin/api ./cmd/api
	chmod +x bin/api

	go build -o bin/cli ./cmd/cli
	chmod +x bin/cli

deploy-lambda-ecr:
	GOARCH=arm64 GOOS=linux go build -tags lambda.norpc -o ./bin/bootstrap ./cmd/lambda
	cd bin && zip lambda.zip ./bootstrap

	aws lambda update-function-code --function-name vo2 \
		--zip-file fileb://bin/lambda.zip

run-api:
	air --build.cmd "make build" --build.bin "./bin/api"
