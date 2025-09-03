package activity

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ActivityTag struct {
	ID          int          `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Description string       `json:"description" db:"description"`
	CreatedAt   time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt   sql.NullTime `json:"updatedAt" db:"updated_at"`
	DeletedAt   sql.NullTime `json:"deletedAt" db:"deleted_at"`
}

type enduranceOutdoorActivityTag struct {
	ActivityID uuid.UUID `db:"activity_id"`
	TagID      int       `db:"tag_id"`
}

type activityTagRepo struct {
	db *sqlx.DB
}

func NewActivityTagRepo(db *sqlx.DB) *activityTagRepo {
	return &activityTagRepo{db: db}
}

func (r *activityTagRepo) Upsert(ctx context.Context, tags []*ActivityTag) ([]*ActivityTag, error) {
	query := `
	INSERT INTO vo2.activity_tags
		(name, description)
	VALUES
		(:name, :description)
	ON CONFLICT (name)
	DO UPDATE SET
		description = EXCLUDED.description`

	_, err := r.db.NamedExecContext(ctx, query, tags)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, t := range tags {
		names = append(names, t.Name)
	}

	insertedTags, err := r.Get(ctx, names)
	if err != nil {
		return nil, err
	}

	return insertedTags, nil
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

func (r *activityTagRepo) TagEnduranceOutdoorActivity(ctx context.Context, a *EnduranceOutdoorActivity, tags []*ActivityTag) error {
	var rows []enduranceOutdoorActivityTag

	for _, t := range tags {
		rows = append(rows, enduranceOutdoorActivityTag{ActivityID: a.ID, TagID: t.ID})
	}

	query := `
	INSERT INTO vo2.activities_endurance_outdoor_tags
		(activity_id, tag_id)
	VALUES
		(:activity_id, :tag_id)`
	_, err := r.db.NamedExecContext(ctx, query, rows)
	if err != nil {
		return err
	}

	return nil
}
