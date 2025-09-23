package activity

import (
	"context"
	"time"

	"github.com/gabrieleangeletti/vo2/internal/generated/models"
	"github.com/jmoiron/sqlx"
)

type ActivityTag struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitzero"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero"`
	DeletedAt   time.Time `json:"deletedAt,omitzero"`
}

func NewActivityTag(t models.Vo2ActivityTag) *ActivityTag {
	return &ActivityTag{
		ID:          int(t.ID),
		Name:        t.Name,
		Description: t.Description.String,
		CreatedAt:   t.CreatedAt.Time,
		UpdatedAt:   t.UpdatedAt.Time,
		DeletedAt:   t.DeletedAt.Time,
	}
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
