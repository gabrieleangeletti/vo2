package internal

import (
	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/util"
)

func DefaultDBConfig() database.Config {
	return database.Config{
		Host:           util.GetSecret("POSTGRES_HOST", true),
		Port:           util.GetSecret("POSTGRES_PORT", true),
		User:           util.GetSecret("POSTGRES_USER", true),
		Password:       util.GetSecret("POSTGRES_PASSWORD", true),
		DB:             util.GetSecret("POSTGRES_DB", true),
		SSLMode:        util.GetSecret("POSTGRES_SSLMODE", true),
		ChannelBinding: util.GetSecret("POSTGRES_CHANNEL_BINDING", true),
	}
}
