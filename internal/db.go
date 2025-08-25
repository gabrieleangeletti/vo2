package internal

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DB       string
}

func NewDB(cfg DBConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DB,
	)

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func DefaultDBConfig() DBConfig {
	return DBConfig{
		Host:     GetSecret("POSTGRES_HOST", true),
		Port:     GetSecret("POSTGRES_PORT", true),
		User:     GetSecret("POSTGRES_USER", true),
		Password: GetSecret("POSTGRES_PASS", true),
		DB:       GetSecret("POSTGRES_DB", true),
	}
}
