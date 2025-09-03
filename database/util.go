package database

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type Config struct {
	Host           string
	Port           string
	User           string
	Password       string
	DB             string
	SSLMode        string
	ChannelBinding string
}

func NewDB(cfg Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&channel_binding=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DB,
		cfg.SSLMode,
		cfg.ChannelBinding,
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

// IDB is an interface meant to represent the union type sqlx.DB | sqlx.Tx
type IDB interface {
	Get(dest any, query string, args ...any) error
	NamedExec(query string, arg any) (sql.Result, error)
}

func ToNullInt16[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float64](val T) sql.NullInt16 {
	if val == 0 {
		return sql.NullInt16{Int16: 0, Valid: false}
	}

	return sql.NullInt16{Int16: int16(val), Valid: true}
}

func ToNullInt32[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float64](val T) sql.NullInt32 {
	if val == 0 {
		return sql.NullInt32{Int32: 0, Valid: false}
	}

	return sql.NullInt32{Int32: int32(val), Valid: true}
}

func ToNullString(val string) sql.NullString {
	if val == "" {
		return sql.NullString{String: "", Valid: false}
	}

	return sql.NullString{String: val, Valid: true}
}