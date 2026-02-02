package models

import (
	"errors"
	"time"
)

type Metric struct {
	Timestamp string  `json:"timestamp"`
	DeviceID  string  `json:"device_id"`
	CPU       float64 `json:"cpu"`
	RPS       float64 `json:"rps"`
}

func (m *Metric) Validate() error {
	if m.DeviceID == "" {
		return errors.New("device_id is required")
	}

	if m.Timestamp == "" {
		return errors.New("timestamp is required")
	}

	if _, err := time.Parse(time.RFC3339, m.Timestamp); err != nil {
		return errors.New("invalid timestamp format, expected RFC3339")
	}

	if m.CPU < 0 || m.CPU > 1 {
		return errors.New("cpu must be between 0 and 1")
	}

	if m.RPS < 0 {
		return errors.New("rps must be non-negative")
	}

	return nil
}

func (m *Metric) ProcessedAt() time.Time {
	t, err := time.Parse(time.RFC3339, m.Timestamp)
	if err != nil {
		return time.Now()
	}
	return t
}

type AnalysisResult struct {
	DeviceID          string    `json:"device_id"`
	RollingAverageRPS float64   `json:"rolling_average_rps"`
	IsAnomaly         bool      `json:"is_anomaly"`
	ZScore            float64   `json:"z_score"`
	ProcessedAt       time.Time `json:"processed_at"`
}
