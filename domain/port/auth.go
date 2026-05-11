// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

// IUserRepo is the outbound port for user persistence.
type IUserRepo interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByLogin(ctx context.Context, login string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	UpsertOAuth(ctx context.Context, u *model.User) error
	List(ctx context.Context) ([]*model.User, error)
	Update(ctx context.Context, u *model.User) error
	SetPassword(ctx context.Context, id int64, hash string) error
	Deactivate(ctx context.Context, id int64) error
	SetLastLogin(ctx context.Context, id int64) error
	Count(ctx context.Context) (int, error)
}

// ITokenRepo is the outbound port for API token persistence.
type ITokenRepo interface {
	Create(ctx context.Context, t *model.Token) error
	GetByHash(ctx context.Context, hash string) (*model.Token, error)
	ListByUser(ctx context.Context, userID int64) ([]*model.Token, error)
	Delete(ctx context.Context, id int64) error
	UpdateLastUsed(ctx context.Context, id int64) error
}

// IOAuthProvider is the outbound port for OAuth identity providers.
type IOAuthProvider interface {
	// AuthURL returns the redirect URL to begin the OAuth flow.
	AuthURL(state string) string
	// Exchange completes the OAuth flow and returns the authenticated user's identity.
	Exchange(ctx context.Context, code string) (*model.OAuthUser, error)
}
