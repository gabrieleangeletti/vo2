# Doppler configurations
VAR_dev_DOPPLER_CFG:=dev
VAR_staging_DOPPLER_CFG:=stg
VAR_prod_DOPPLER_CFG:=prd

cli: build
	doppler -c ${VAR_$(env)_DOPPLER_CFG} run -- ./bin/cli $(args)

db-migration-create:
	./db/create-migration.sh $(name)

db-migration-apply:
	doppler -c ${VAR_$(env)_DOPPLER_CFG} run -- ./db/apply-migration.sh $(op)

build:
	go build -o bin/api ./cmd/api
	chmod +x bin/api

	go build -o bin/cli ./cmd/cli
	chmod +x bin/cli

build-lambda:
	GOARCH=arm64 GOOS=linux go build -tags lambda.norpc -o ./bin/bootstrap ./cmd/lambda
	cd bin && zip lambda.zip ./bootstrap

tf-init:
	cd infra && terraform init -backend-config="bucket=$(state_bucket)"

tf-plan:
	cd infra && terraform plan -var="doppler_workspace_id=$$(doppler settings --json | jq -r .id)" -var="doppler_secret_name=$$(doppler -c prd secrets get AWS_DOPPLER_SECRET_NAME --plain)"

tf-apply: build-lambda
	cd infra && terraform apply -auto-approve -var="doppler_workspace_id=$$(doppler settings --json | jq -r .id)" -var="doppler_secret_name=$$(doppler -c prd secrets get AWS_DOPPLER_SECRET_NAME --plain)"
	doppler -c prd secrets set API_BASE_URL=$$(cd infra && terraform output -raw lambda_function_url)

run-api:
	air --build.cmd "make build" --build.bin "doppler -c ${VAR_$(env)_DOPPLER_CFG} run -- ./bin/api"
