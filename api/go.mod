module github.com/gabrieleangeletti/vo2/api

go 1.25.0

replace github.com/gabrieleangeletti/vo2/core v0.0.0 => ../core

require (
	github.com/gabrieleangeletti/stride v0.0.1
	github.com/gabrieleangeletti/vo2/core v0.0.0
	github.com/jmoiron/sqlx v1.4.0
)
