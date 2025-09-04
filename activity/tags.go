package activity

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type ActivityTag struct {
	ID          int            `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	Description sql.NullString `json:"description" db:"description"`
	CreatedAt   time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt   sql.NullTime   `json:"updatedAt" db:"updated_at"`
	DeletedAt   sql.NullTime   `json:"deletedAt" db:"deleted_at"`
}

type activityTagRepo struct {
	db *sqlx.DB
}

func NewActivityTagRepo(db *sqlx.DB) *activityTagRepo {
	return &activityTagRepo{db: db}
}

func (r *activityTagRepo) Get(ctx context.Context, names []string) ([]*ActivityTag, error) {
	var rows []*ActivityTag

	query, args, err := sqlx.In(`SELECT * FROM vo2.activity_tags WHERE name IN (?)`, names)
	if err != nil {
		return nil, err
	}

	query = r.db.Rebind(query)

	err = r.db.SelectContext(ctx, &rows, query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
