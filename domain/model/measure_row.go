package model

import "time"

// MeasureRow is the database representation of a single metric measure.
type MeasureRow struct {
	ID            int64     `json:"id"`
	ScanID        int64     `json:"scan_id"`
	ProjectID     int64     `json:"project_id"`
	MetricKey     string    `json:"metric_key"`
	ComponentPath string    `json:"component_path"`
	Value         float64   `json:"value"`
	CreatedAt     time.Time `json:"created_at"`
}

// TrendPoint is a single data point used for metric trend graphs.
type TrendPoint struct {
	AnalysisDate time.Time `json:"analysis_date"`
	Value        float64   `json:"value"`
}
