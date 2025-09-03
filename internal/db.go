package internal

import (
	"github.com/gabrieleangeletti/vo2/database"
)

func DefaultDBConfig() database.Config {
	return database.Config{
		Host:           GetSecret("POSTGRES_HOST", true),
		Port:           GetSecret("POSTGRES_PORT", true),
		User:           GetSecret("POSTGRES_USER", true),
		Password:       GetSecret("POSTGRES_PASSWORD", true),
		DB:             GetSecret("POSTGRES_DB", true),
		SSLMode:        GetSecret("POSTGRES_SSLMODE", true),
		ChannelBinding: GetSecret("POSTGRES_CHANNEL_BINDING", true),
	}
}
