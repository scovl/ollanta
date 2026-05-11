package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Webhook is a registered notification endpoint.
type Webhook struct {
	ID        int64     `json:"id"`
	ProjectID *int64    `json:"project_id,omitempty"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookDelivery records a single delivery attempt.
type WebhookDelivery struct {
	ID           int64     `json:"id"`
	WebhookID    int64     `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      []byte    `json:"payload"`
	ResponseCode *int      `json:"response_code,omitempty"`
	ResponseBody *string   `json:"response_body,omitempty"`
	Success      bool      `json:"success"`
	Attempt      int       `json:"attempt"`
	DeliveredAt  time.Time `json:"delivered_at"`
}

// WebhookRepository provides CRUD for webhooks and deliveries.
type WebhookRepository struct {
	db *DB
}

// NewWebhookRepository creates a WebhookRepository backed by db.
func NewWebhookRepository(db *DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// List returns all webhooks. Pass projectID=0 to list global webhooks.
func (r *WebhookRepository) List(ctx context.Context, projectID int64) ([]*Webhook, error) {
	var rows pgx.Rows
	var err error
	if projectID == 0 {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, project_id, name, url, secret, events, enabled, created_at, updated_at
			FROM webhooks WHERE project_id IS NULL ORDER BY name`)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, project_id, name, url, secret, events, enabled, created_at, updated_at
			FROM webhooks WHERE project_id = $1 OR project_id IS NULL ORDER BY name`, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

// GetByID returns a single webhook.
func (r *WebhookRepository) GetByID(ctx context.Context, id int64) (*Webhook, error) {
	wh := &Webhook{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, project_id, name, url, secret, events, enabled, created_at, updated_at
		FROM webhooks WHERE id = $1`, id,
	).Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret, &wh.Events,
		&wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return wh, err
}

// Create inserts a new webhook.
func (r *WebhookRepository) Create(ctx context.Context, wh *Webhook) error {
	if wh.Events == nil {
		wh.Events = []string{}
	}
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO webhooks (project_id, name, url, secret, events, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		wh.ProjectID, wh.Name, wh.URL, wh.Secret, wh.Events, wh.Enabled,
	).Scan(&wh.ID, &wh.CreatedAt, &wh.UpdatedAt)
}

// Update updates a webhook's mutable fields.
func (r *WebhookRepository) Update(ctx context.Context, wh *Webhook) error {
	if wh.Events == nil {
		wh.Events = []string{}
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE webhooks
		SET name = $1, url = $2, secret = $3, events = $4, enabled = $5, updated_at = now()
		WHERE id = $6`,
		wh.Name, wh.URL, wh.Secret, wh.Events, wh.Enabled, wh.ID)
	return err
}

// Delete removes a webhook.
func (r *WebhookRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
	return err
}

// CreateDelivery records a delivery attempt.
func (r *WebhookRepository) CreateDelivery(ctx context.Context, d *WebhookDelivery) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (webhook_id, event, payload, response_code, response_body, success, attempt)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, delivered_at`,
		d.WebhookID, d.Event, d.Payload, d.ResponseCode, d.ResponseBody, d.Success, d.Attempt,
	).Scan(&d.ID, &d.DeliveredAt)
}

// ListDeliveries returns recent deliveries for a webhook (newest first).
func (r *WebhookRepository) ListDeliveries(ctx context.Context, webhookID int64, limit int) ([]*WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, webhook_id, event, payload, response_code, response_body, success, attempt, delivered_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY delivered_at DESC
		LIMIT $2`, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

// ForEvent returns all enabled webhooks that subscribe to the given event (global + per-project).
func (r *WebhookRepository) ForEvent(ctx context.Context, projectID int64, event string) ([]*Webhook, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, project_id, name, url, secret, events, enabled, created_at, updated_at
		FROM webhooks
		WHERE enabled = TRUE
		  AND (project_id IS NULL OR project_id = $1)
		  AND (events = '{}' OR $2 = ANY(events))`,
		projectID, event)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanWebhooks(rows pgx.Rows) ([]*Webhook, error) {
	var out []*Webhook
	for rows.Next() {
		wh := &Webhook{}
		if err := rows.Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.URL, &wh.Secret,
			&wh.Events, &wh.Enabled, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, wh)
	}
	return out, rows.Err()
}

func scanDeliveries(rows pgx.Rows) ([]*WebhookDelivery, error) {
	var out []*WebhookDelivery
	for rows.Next() {
		d := &WebhookDelivery{}
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload,
			&d.ResponseCode, &d.ResponseBody, &d.Success, &d.Attempt, &d.DeliveredAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
