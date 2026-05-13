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

// BulkInsert inserts all rows using the COPY protocol (50x faster than individual INSERTs).
func (r *MeasureRepository) BulkInsert(ctx context.Context, measures []MeasureRow) error {
	if len(measures) == 0 {
		return nil
	}
	conn, err := r.db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for bulk insert: %w", err)
	}
	defer conn.Release()

	rows := make([][]interface{}, len(measures))
	for i, m := range measures {
		rows[i] = []interface{}{m.ScanID, m.ProjectID, m.MetricKey, m.ComponentPath, m.Value}
	}
	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"measures"},
		[]string{"scan_id", "project_id", "metric_key", "component_path", "value"},
		pgx.CopyFromRows(rows),
	)
	return err
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
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
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

// TrendForComponent returns a time-ordered series of component-level metric values.
func (r *MeasureRepository) TrendForComponent(ctx context.Context, projectID int64, metricKey, componentPath string, from, to time.Time) ([]TrendPoint, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT s.analysis_date, m.value
		FROM measures m
		JOIN scans s ON s.id = m.scan_id
		WHERE m.project_id = $1
		  AND m.metric_key = $2
		  AND m.component_path = $3
		  AND s.analysis_date BETWEEN $4 AND $5
		ORDER BY s.analysis_date ASC`, projectID, metricKey, componentPath, from, to,
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

// UpsertLive atomically inserts or updates a live measure value.
// Uses ON CONFLICT to avoid race conditions between parallel workers.
func (r *MeasureRepository) UpsertLive(ctx context.Context, projectID int64, scanID int64, metricKey string, componentPath string, value float64) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO live_measures (project_id, component_path, metric_key, value, scan_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (project_id, component_path, metric_key)
		DO UPDATE SET value = EXCLUDED.value, scan_id = EXCLUDED.scan_id, updated_at = now()`,
		projectID, componentPath, metricKey, value, scanID)
	return err
}

// UpsertLiveBatch upserts multiple live measures in a single round-trip.
func (r *MeasureRepository) UpsertLiveBatch(ctx context.Context, projectID int64, scanID int64, metrics map[string]float64) error {
	if len(metrics) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metrics))
	values := make([]float64, 0, len(metrics))
	for k, v := range metrics {
		keys = append(keys, k)
		values = append(values, v)
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO live_measures (project_id, component_path, metric_key, value, scan_id, updated_at)
		SELECT $1, '', unnest($2::text[]), unnest($3::numeric[]), $4, now()
		ON CONFLICT (project_id, component_path, metric_key)
		DO UPDATE SET value = EXCLUDED.value, scan_id = EXCLUDED.scan_id, updated_at = now()`,
		projectID, keys, values, scanID)
	return err
}

// GetLive returns all current measure values for a project.
func (r *MeasureRepository) GetLive(ctx context.Context, projectID int64) (map[string]float64, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT metric_key, value FROM live_measures
		WHERE project_id = $1 AND component_path = ''`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	measures := make(map[string]float64)
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		measures[key] = value
	}
	return measures, rows.Err()
}

// UpsertDailyAggregate atomically updates daily rollup for a metric.
func (r *MeasureRepository) UpsertDailyAggregate(ctx context.Context, projectID int64, metricKey string, date string, value float64) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO measure_daily_aggregates (project_id, metric_key, date, value_avg, value_max, value_min, sample_count)
		VALUES ($1, $2, $3::date, $4, $4, $4, 1)
		ON CONFLICT (project_id, metric_key, date)
		DO UPDATE SET
			value_avg = (measure_daily_aggregates.value_avg * measure_daily_aggregates.sample_count + EXCLUDED.value_avg)
			            / (measure_daily_aggregates.sample_count + 1),
			value_max = GREATEST(measure_daily_aggregates.value_max, EXCLUDED.value_max),
			value_min = LEAST(measure_daily_aggregates.value_min, EXCLUDED.value_min),
			sample_count = measure_daily_aggregates.sample_count + 1,
			updated_at = now()`,
		projectID, metricKey, date, value)
	return err
}

// UpsertDailyAggregateBatch upserts multiple daily aggregates in a single round-trip.
func (r *MeasureRepository) UpsertDailyAggregateBatch(ctx context.Context, projectID int64, date string, metrics map[string]float64) error {
	if len(metrics) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metrics))
	values := make([]float64, 0, len(metrics))
	for k, v := range metrics {
		keys = append(keys, k)
		values = append(values, v)
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO measure_daily_aggregates (project_id, metric_key, date, value_avg, value_max, value_min, sample_count)
		SELECT $1, unnest($2::text[]), $3::date, unnest($4::numeric[]), unnest($4::numeric[]), unnest($4::numeric[]), 1
		ON CONFLICT (project_id, metric_key, date)
		DO UPDATE SET
			value_avg = (measure_daily_aggregates.value_avg * measure_daily_aggregates.sample_count + EXCLUDED.value_avg)
			            / (measure_daily_aggregates.sample_count + 1),
			value_max = GREATEST(measure_daily_aggregates.value_max, EXCLUDED.value_max),
			value_min = LEAST(measure_daily_aggregates.value_min, EXCLUDED.value_min),
			sample_count = measure_daily_aggregates.sample_count + 1,
			updated_at = now()`,
		projectID, keys, date, values)
	return err
}

// GetDailyAggregates returns rollup values for trend visualization.
func (r *MeasureRepository) GetDailyAggregates(ctx context.Context, projectID int64, metricKey string, days int) ([]TrendPoint, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT date, value_avg FROM measure_daily_aggregates
		WHERE project_id = $1 AND metric_key = $2
		  AND date >= current_date - $3::integer
		ORDER BY date`, projectID, metricKey, days)
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
