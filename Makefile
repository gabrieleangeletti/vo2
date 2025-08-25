include .env
export $(shell sed 's/=.*//' .env)

db-migration-create:
	./db/create-migration.sh $(name)

db-migration-apply:
	./db/apply-migration.sh $(op) $(sslmode)

run-api:
	air --build.cmd "./build.sh" --build.bin "./api/bin/vo2"
