package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MeasureRow is the database representation of a single metric value.
type MeasureRow struct {
	ID            int64
	ScanID        int64
	ProjectID     int64
	MetricKey     string
	ComponentPath string
	Value         float64
	CreatedAt     time.Time
}

// TrendPoint is a (date, value) pair used in trend queries.
type TrendPoint struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}

// MeasureRepository provides access to the measures table.
type MeasureRepository struct {
	db *DB
}

// NewMeasureRepository creates a MeasureRepository backed by db.
func NewMeasureRepository(db *DB) *MeasureRepository {
	return &MeasureRepository{db: db}
}

// BulkInsert inserts all rows using individual INSERTs within a single transaction.
func (r *MeasureRepository) BulkInsert(ctx context.Context, measures []MeasureRow) error {
	if len(measures) == 0 {
		return nil
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, m := range measures {
		if _, err := tx.Exec(ctx, `
			INSERT INTO measures (scan_id, project_id, metric_key, component_path, value)
			VALUES ($1, $2, $3, $4, $5)`,
			m.ScanID, m.ProjectID, m.MetricKey, m.ComponentPath, m.Value,
		); err != nil {
			return fmt.Errorf("insert measure %s: %w", m.MetricKey, err)
		}
	}
	return tx.Commit(ctx)
}

// GetLatest returns the most recent project-level value for a metric key.
func (r *MeasureRepository) GetLatest(ctx context.Context, projectID int64, metricKey string) (*MeasureRow, error) {
	m := &MeasureRow{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT m.id, m.scan_id, m.project_id, m.metric_key, m.component_path, m.value, m.created_at
		FROM measures m
		JOIN scans s ON s.id = m.scan_id
		WHERE m.project_id = $1
		  AND m.metric_key = $2
		  AND m.component_path = ''
		ORDER BY s.analysis_date DESC
		LIMIT 1`, projectID, metricKey,
	).Scan(&m.ID, &m.ScanID, &m.ProjectID, &m.MetricKey, &m.ComponentPath, &m.Value, &m.CreatedAt)
	return m, err
}

// GetForScan returns a project-level metric value for a specific scan.
func (r *MeasureRepository) GetForScan(ctx context.Context, scanID int64, metricKey string) (*MeasureRow, error) {
	m := &MeasureRow{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, scan_id, project_id, metric_key, component_path, value, created_at
		FROM measures
		WHERE scan_id = $1
		  AND metric_key = $2
		  AND component_path = ''
		LIMIT 1`, scanID, metricKey,
	).Scan(&m.ID, &m.ScanID, &m.ProjectID, &m.MetricKey, &m.ComponentPath, &m.Value, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// GetForScanComponent returns a component-level metric value for a specific scan and path.
func (r *MeasureRepository) GetForScanComponent(ctx context.Context, scanID int64, metricKey, componentPath string) (*MeasureRow, error) {
	m := &MeasureRow{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, scan_id, project_id, metric_key, component_path, value, created_at
		FROM measures
		WHERE scan_id = $1
		  AND metric_key = $2
		  AND component_path = $3
		LIMIT 1`, scanID, metricKey, componentPath,
	).Scan(&m.ID, &m.ScanID, &m.ProjectID, &m.MetricKey, &m.ComponentPath, &m.Value, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// ListForScanMetric returns component-level metric values for a scan.
func (r *MeasureRepository) ListForScanMetric(ctx context.Context, scanID int64, metricKey string, limit int) ([]*MeasureRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, scan_id, project_id, metric_key, component_path, value, created_at
		FROM measures
		WHERE scan_id = $1
		  AND metric_key = $2
		  AND component_path <> ''
		ORDER BY value ASC, component_path ASC
		LIMIT $3`, scanID, metricKey, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var measures []*MeasureRow
	for rows.Next() {
		measure := &MeasureRow{}
		if err := rows.Scan(&measure.ID, &measure.ScanID, &measure.ProjectID, &measure.MetricKey, &measure.ComponentPath, &measure.Value, &measure.CreatedAt); err != nil {
			return nil, err
		}
		measures = append(measures, measure)
	}
	return measures, rows.Err()
}

// Trend returns a time-ordered series of project-level metric values between from and to.
func (r *MeasureRepository) Trend(ctx context.Context, projectID int64, metricKey string, from, to time.Time) ([]TrendPoint, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT s.analysis_date, m.value
		FROM measures m
		JOIN scans s ON s.id = m.scan_id
		WHERE m.project_id = $1
		  AND m.metric_key = $2
		  AND m.component_path = ''
		  AND s.analysis_date BETWEEN $3 AND $4
		ORDER BY s.analysis_date ASC`, projectID, metricKey, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TrendPoint
	for rows.Next() {
		var pt TrendPoint
		if err := rows.Scan(&pt.Date, &pt.Value); err != nil {
			return nil, err
		}
		points = append(points, pt)
	}
	return points, rows.Err()
}
