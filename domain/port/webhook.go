// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

// IWebhookRepo is the outbound port for webhook persistence.
type IWebhookRepo interface {
	Create(ctx context.Context, w *model.Webhook) error
	GetByID(ctx context.Context, id int64) (*model.Webhook, error)
	ListByProject(ctx context.Context, projectID int64) ([]*model.Webhook, error)
	Update(ctx context.Context, w *model.Webhook) error
	Delete(ctx context.Context, id int64) error
	RecordDelivery(ctx context.Context, d *model.WebhookDelivery) error
	ListDeliveries(ctx context.Context, webhookID int64, limit int) ([]*model.WebhookDelivery, error)
}

// IWebhookDispatcher is an optional outbound port for firing webhooks.
type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, projectID, scanID int64, event string) error
}
