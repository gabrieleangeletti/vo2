package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/provider"
)

type ProviderActivityRawData struct {
	ID                  uuid.UUID       `json:"id" db:"id"`
	ProviderID          int             `json:"providerId" db:"provider_id"`
	UserID              uuid.UUID       `json:"userId" db:"user_id"`
	ProviderActivityID  string          `json:"providerActivityId" db:"provider_activity_id"`
	StartTime           time.Time       `json:"startTime" db:"start_time"`
	ElapsedTime         int             `json:"elapsedTime" db:"elapsed_time"`
	IanaTimezone        sql.NullString  `json:"ianaTimezone" db:"iana_timezone"`
	UTCOffset           sql.NullInt32   `json:"utcOffset" db:"utc_offset"`
	Data                json.RawMessage `json:"data" db:"data"`
	DetailedActivityURI sql.NullString  `json:"detailedActivityUri" db:"detailed_activity_uri"`
	CreatedAt           time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt           sql.NullTime    `json:"updatedAt" db:"updated_at"`
	DeletedAt           sql.NullTime    `json:"deletedAt" db:"deleted_at"`
}

func (a *ProviderActivityRawData) ToEnduranceOutdoorActivity(providerMap map[int]provider.Provider) (*EnduranceOutdoorActivity, error) {
	prov, ok := providerMap[a.ProviderID]
	if !ok {
		return nil, fmt.Errorf("%w: %d", provider.ErrProviderNotFound, a.ProviderID)
	}

	switch prov.Slug {
	case "strava":
		var activity strava.ActivitySummary
		err := json.Unmarshal(a.Data, &activity)
		if err != nil {
			return nil, err
		}

		act, err := activity.ToEnduranceActivity()
		if err != nil {
			return nil, err
		}

		enduranceOutdoorActivity := &EnduranceOutdoorActivity{
			ProviderID:            a.ProviderID,
			UserID:                a.UserID,
			ProviderRawActivityID: a.ID,
			Name:                  activity.Name,
			Description:           database.ToNullString(activity.Description),
			Sport:                 act.Sport,
			StartTime:             a.StartTime,
			EndTime:               a.StartTime.Add(time.Duration(a.ElapsedTime) * time.Second),
			IanaTimezone:          a.IanaTimezone,
			UTCOffset:             a.UTCOffset,
			ElapsedTime:           act.ElapsedTime,
			MovingTime:            act.MovingTime,
			Distance:              int(act.Distance),
			AvgSpeed:              act.AvgSpeed,
		}

		if act.ElevGain != nil {
			enduranceOutdoorActivity.ElevGain = database.ToNullInt32(*act.ElevGain)
		}

		if act.ElevLoss != nil {
			enduranceOutdoorActivity.ElevLoss = database.ToNullInt32(*act.ElevLoss)
		}

		if act.AvgHR != nil {
			enduranceOutdoorActivity.AvgHR = database.ToNullInt16(*act.AvgHR)
		}

		if act.MaxHR != nil {
			enduranceOutdoorActivity.MaxHR = database.ToNullInt16(*act.MaxHR)
		}

		if activity.SummaryPolyline() != "" {
			enduranceOutdoorActivity.SummaryPolyline = database.ToNullString(activity.SummaryPolyline())

			wkt, err := stride.PolylineToWKT(activity.SummaryPolyline())
			if err != nil {
				return nil, err
			}

			enduranceOutdoorActivity.SummaryRoute = database.ToNullString(wkt)
		}

		return enduranceOutdoorActivity, nil
	default:
		return nil, fmt.Errorf("%w: %d", provider.ErrUnsupportedProvider, a.ProviderID)
	}
}

func (a *ProviderActivityRawData) Save(db *sqlx.DB) error {
	_, err := db.NamedExec(`
	INSERT INTO vo2.provider_activity_raw_data
		(provider_id, user_id, provider_activity_id, start_time, elapsed_time, iana_timezone, utc_offset, data, detailed_activity_uri)
	VALUES
		(:provider_id, :user_id, :provider_activity_id, :start_time, :elapsed_time, :iana_timezone, :utc_offset, :data, :detailed_activity_uri)
	ON CONFLICT
		(provider_id, user_id, provider_activity_id)
	DO UPDATE SET
		start_time = :start_time,
		elapsed_time = :elapsed_time,
		iana_timezone = :iana_timezone,
		utc_offset = :utc_offset,
		data = :data,
		detailed_activity_uri = :detailed_activity_uri
	`, a)
	if err != nil {
		return err
	}

	return nil
}

func GetProviderActivityRawData(db *sqlx.DB, providerID int, userID uuid.UUID) ([]*ProviderActivityRawData, error) {
	var data []*ProviderActivityRawData

	err := db.Select(&data, `
	SELECT * FROM vo2.provider_activity_raw_data
	WHERE provider_id = $1 AND user_id = $2
	`, providerID, userID)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type EnduranceOutdoorActivity struct {
	ID                    uuid.UUID      `json:"id" db:"id"`
	ProviderID            int            `json:"providerId" db:"provider_id"`
	UserID                uuid.UUID      `json:"userId" db:"user_id"`
	ProviderRawActivityID uuid.UUID      `json:"providerRawActivityId" db:"provider_raw_activity_id"`
	Name                  string         `json:"name" db:"name"`
	Description           sql.NullString `json:"description" db:"description"`
	Sport                 stride.Sport   `json:"sport" db:"sport"`
	StartTime             time.Time      `json:"startTime" db:"start_time"`
	EndTime               time.Time      `json:"endTime" db:"end_time"`
	IanaTimezone          sql.NullString `json:"ianaTimezone" db:"iana_timezone"`
	UTCOffset             sql.NullInt32  `json:"utcOffset" db:"utc_offset"`
	ElapsedTime           int            `json:"elapsedTime" db:"elapsed_time"`
	MovingTime            int            `json:"movingTime" db:"moving_time"`
	Distance              int            `json:"distance" db:"distance"`
	ElevGain              sql.NullInt32  `json:"elevGain" db:"elev_gain"`
	ElevLoss              sql.NullInt32  `json:"elevLoss" db:"elev_loss"`
	AvgSpeed              float64        `json:"avgSpeed" db:"avg_speed"`
	AvgHR                 sql.NullInt16  `json:"avgHR" db:"avg_hr"`
	MaxHR                 sql.NullInt16  `json:"maxHR" db:"max_hr"`
	SummaryPolyline       sql.NullString `json:"summaryPolyline" db:"summary_polyline"`
	SummaryRoute          sql.NullString `json:"summaryRoute" db:"summary_route"`
	CreatedAt             time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt             sql.NullTime   `json:"updatedAt" db:"updated_at"`
	DeletedAt             sql.NullTime   `json:"deletedAt" db:"deleted_at"`

	Provider *provider.Data `json:"provider" db:"provider"`
	Tags     []*ActivityTag `json:"tags" db:"tags"`
}

// Tags are extracted from the activity name.
// Any string contained in square brakets, e.g. [my-tag] is considered a tag.
// An exception is for numbers, e.g. [13], which are ignored.
func (a *EnduranceOutdoorActivity) ActivityTags() []*ActivityTag {
	var tags []*ActivityTag
	re := regexp.MustCompile(`\[(.*?)\]`)

	matches := re.FindAllStringSubmatch(a.Name, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tag := match[1]

			if _, err := strconv.Atoi(tag); err == nil {
				continue
			}
			tags = append(tags, &ActivityTag{Name: tag})
		}
	}

	return tags
}

type enduranceOutdoorActivityRepo struct {
	db *sqlx.DB
}

func NewEnduranceOutdoorActivityRepo(db *sqlx.DB) *enduranceOutdoorActivityRepo {
	return &enduranceOutdoorActivityRepo{db: db}
}

func (r *enduranceOutdoorActivityRepo) Upsert(ctx context.Context, a *EnduranceOutdoorActivity) (uuid.UUID, error) {
	var id uuid.UUID

	err := r.db.QueryRowxContext(ctx, `
	INSERT INTO vo2.activities_endurance_outdoor
		(provider_id, user_id, provider_raw_activity_id, sport, start_time, end_time, iana_timezone, utc_offset, elapsed_time, moving_time, distance, elev_gain, elev_loss, avg_speed, avg_hr, max_hr, summary_polyline, summary_route)
	VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	ON CONFLICT
		(provider_id, user_id, provider_raw_activity_id)
	DO UPDATE SET
		sport = $4,
		start_time = $5,
		end_time = $6,
		iana_timezone = $7,
		utc_offset = $8,
		elapsed_time = $9,
		moving_time = $10,
		distance = $11,
		elev_gain = $12,
		elev_loss = $13,
		avg_speed = $14,
		avg_hr = $15,
		max_hr = $16,
		summary_polyline = $17,
		summary_route = $18
	RETURNING id
	`,
		a.ProviderID, a.UserID, a.ProviderRawActivityID, a.Sport, a.StartTime, a.EndTime, a.IanaTimezone, a.UTCOffset, a.ElapsedTime, a.MovingTime, a.Distance, a.ElevGain, a.ElevLoss, a.AvgSpeed, a.AvgHR, a.MaxHR, a.SummaryPolyline, a.SummaryRoute,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (r *enduranceOutdoorActivityRepo) Get(ctx context.Context, id int64) (*EnduranceOutdoorActivity, error) {
	var row EnduranceOutdoorActivity

	err := r.db.GetContext(ctx, &row, `
	SELECT
		a.*,
		p.id AS "provider.id",
		p.name AS "provider.name",
		p.slug AS "provider.slug",
		t.id AS "tag.id",
		t.name AS "tag.name",
		t.description AS "tag.description"
	FROM vo2.activities_endurance_outdoor a
	JOIN vo2.providers p ON a.provider_id = p.id
	JOIN vo2.activities_endurance_outdoor_tags at ON at.activity_id = a.id
	JOIN vo2.activity_tags t ON at.tag_id = t.id
	WHERE a.id = $1
	`, id)
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *enduranceOutdoorActivityRepo) ListByUser(ctx context.Context, providerID int, userID uuid.UUID) ([]*EnduranceOutdoorActivity, error) {
	var rows []*EnduranceOutdoorActivity

	err := r.db.SelectContext(ctx, &rows, `
	SELECT
		a.*,
		p.id AS "provider.id",
		p.name AS "provider.name",
		p.slug AS "provider.slug",
		t.id AS "tag.id",
		t.name AS "tag.name",
		t.description AS "tag.description"
	FROM vo2.activities_endurance_outdoor a
	JOIN vo2.providers p ON a.provider_id = p.id
	JOIN vo2.activities_endurance_outdoor_tags at ON at.activity_id = a.id
	JOIN vo2.activity_tags t ON at.tag_id = t.id
	WHERE a.provider_id = $1 AND a.user_id = $2
	`, providerID, userID)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *enduranceOutdoorActivityRepo) ListByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*EnduranceOutdoorActivity, error) {
	var rows []*EnduranceOutdoorActivity

	err := r.db.SelectContext(ctx, &rows, `
	SELECT
		a.*,
		p.id AS "provider.id",
		p.name AS "provider.name",
		p.slug AS "provider.slug",
		t.id AS "tag.id",
		t.name AS "tag.name",
		t.description AS "tag.description"
	FROM vo2.activities_endurance_outdoor a
	JOIN vo2.providers p ON a.provider_id = p.id
	JOIN vo2.activities_endurance_outdoor_tags at ON at.activity_id = a.id
	JOIN vo2.activity_tags t ON at.tag_id = t.id
	WHERE
		a.provider_id = $1 AND
		a.user_id = $2 AND
		t.name = $3
	`, providerID, userID)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (r *enduranceOutdoorActivityRepo) ListBySport(ctx context.Context, sport stride.Sport, limit int) ([]*EnduranceOutdoorActivity, error) {
	rows := []*EnduranceOutdoorActivity{}

	q := `
	SELECT
		a.*,
		p.id AS "provider.id",
		p.name AS "provider.name",
		p.slug AS "provider.slug",
		t.id AS "tag.id",
		t.name AS "tag.name",
		t.description AS "tag.description"
	FROM vo2.activities_endurance_outdoor a
	JOIN vo2.providers p ON a.provider_id = p.id
	JOIN vo2.activities_endurance_outdoor_tags at ON at.activity_id = a.id
	JOIN vo2.activity_tags t ON at.tag_id = t.id
	WHERE sport = $1
	ORDER BY start_time DESC
	LIMIT $2`

	err := r.db.SelectContext(ctx, &rows, q, string(sport), limit)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
