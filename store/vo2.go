package store

import (
	"github.com/gabrieleangeletti/vo2"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
)

func newAthleteCurrentMeasurements(m models.Vo2AthleteCurrentMeasurement) *vo2.AthleteCurrentMeasurements {
	return &vo2.AthleteCurrentMeasurements{
		AthleteID:        m.AthleteID,
		Lt1Value:         m.Lt1Value.Float64,
		Lt1MeasuredAt:    m.Lt1MeasuredAt.Time,
		Lt1Timezone:      m.Lt1IanaTimezone.String,
		Lt1Source:        vo2.AthleteMeasurementSource(m.Lt1Source.Vo2AthleteMeasurementSource),
		Lt1Notes:         m.Lt1Notes.String,
		Lt2Value:         m.Lt2Value.Float64,
		Lt2MeasuredAt:    m.Lt2MeasuredAt.Time,
		Lt2Timezone:      m.Lt2IanaTimezone.String,
		Lt2Source:        vo2.AthleteMeasurementSource(m.Lt2Source.Vo2AthleteMeasurementSource),
		Lt2Notes:         m.Lt2Notes.String,
		Vo2maxValue:      m.Vo2maxValue.Float64,
		Vo2maxMeasuredAt: m.Vo2maxMeasuredAt.Time,
		Vo2maxTimezone:   m.Vo2maxIanaTimezone.String,
		Vo2maxSource:     vo2.AthleteMeasurementSource(m.Vo2maxSource.Vo2AthleteMeasurementSource),
		Vo2maxNotes:      m.Vo2maxNotes.String,
		WeightValue:      m.WeightValue.Float64,
		WeightMeasuredAt: m.WeightMeasuredAt.Time,
		WeightTimezone:   m.WeightIanaTimezone.String,
		WeightSource:     vo2.AthleteMeasurementSource(m.WeightSource.Vo2AthleteMeasurementSource),
		WeightNotes:      m.WeightNotes.String,
	}
}
