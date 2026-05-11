package model

import "time"

// Webhook is the canonical webhook record.
type Webhook struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookDelivery records a single webhook delivery attempt.
type WebhookDelivery struct {
	ID           int64     `json:"id"`
	WebhookID    int64     `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      string    `json:"payload"`
	ResponseCode int       `json:"response_code"`
	ResponseBody string    `json:"response_body"`
	Success      bool      `json:"success"`
	Attempt      int       `json:"attempt"`
	DeliveredAt  time.Time `json:"delivered_at"`
}
