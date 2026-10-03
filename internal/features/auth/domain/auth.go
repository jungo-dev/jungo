package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TokenType distinguishes the two tokens of a session.
type TokenType int16

// Token type constants (auth_tokens.type).
const (
	TokenTypeAccess  TokenType = 1
	TokenTypeRefresh TokenType = 2
)

// String returns the token type name used in API responses.
func (t TokenType) String() string {
	switch t {
	case TokenTypeAccess:
		return "access_token"
	case TokenTypeRefresh:
		return "refresh_token"
	default:
		return "unknown"
	}
}

// Revocation reasons (auth_tokens.revoked_reason).
const (
	RevokedReasonLogout       = "logout"
	RevokedReasonRefresh      = "refresh"
	RevokedReasonReuse        = "reuse"
	RevokedReasonAdmin        = "admin"
	RevokedReasonUserDisabled = "user_disabled"
)

// Token status values reported by GetTokenDetails.
const (
	TokenStatusActive  = "active"
	TokenStatusExpired = "expired"
	TokenStatusRevoked = "revoked"
)

// TokenState is the stored state of one token of a session.
type TokenState struct {
	Uuid      uuid.UUID
	Hash      string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// Status reports whether the token is active, expired or revoked at now.
func (t TokenState) Status(now time.Time) string {
	switch {
	case t.RevokedAt != nil:
		return TokenStatusRevoked
	case !now.Before(t.ExpiresAt):
		return TokenStatusExpired
	default:
		return TokenStatusActive
	}
}

// Session is one login session (token family): its user and latest token pair.
type Session struct {
	UserID     int64
	UserUuid   uuid.UUID
	Email      string
	FirstName  string
	LastName   string
	AvatarUrl  *string
	UserActive bool

	FamilyUuid uuid.UUID
	IPAddress  *string
	UserAgent  *string
	DeviceID   *string
	CreatedAt  time.Time

	Access  TokenState
	Refresh TokenState
}

// Identity is the authenticated principal attached to a request by the auth middleware.
type Identity struct {
	UserID     int64
	UserUuid   uuid.UUID
	Email      string
	FamilyUuid uuid.UUID
	TokenUuid  uuid.UUID
	ExpiresAt  time.Time
	// Session is the session the access token belongs to (for /auth/me).
	Session *Session
}

// TokenDetails describes a token for internal (service-to-service) inspection.
type TokenDetails struct {
	Session   *Session
	TokenType TokenType
	Status    string
}

// StoredToken is a single auth_tokens row with its owner's status.
type StoredToken struct {
	Uuid          uuid.UUID
	UserID        int64
	UserUuid      uuid.UUID
	UserActive    bool
	FamilyUuid    uuid.UUID
	Type          TokenType
	DeviceID      *string
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokedReason *string
}

// SessionSummary is one active session in the user's session list.
type SessionSummary struct {
	FamilyUuid uuid.UUID
	IPAddress  *string
	UserAgent  *string
	DeviceID   *string
	CreatedAt  time.Time
	UpdatedAt  *time.Time
	ExpiresAt  time.Time
	Current    bool
}

// Credentials is the login view of a user.
type Credentials struct {
	UserID       int64
	UserUuid     uuid.UUID
	PasswordHash string
	Active       bool
}

// Metadata describes the client making an auth request.
type Metadata struct {
	IPAddress      string
	UserAgent      string
	DeviceID       string
	RecaptchaToken string
}

// LoginInput holds data for Login.
type LoginInput struct {
	Email    string
	Password string
	Metadata Metadata
}

// TokenPair is the sealed token pair returned to the client.
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// NewSessionParams holds the hashed token pair and client metadata to persist.
type NewSessionParams struct {
	UserID           int64
	FamilyUuid       uuid.UUID
	AccessHash       string
	AccessExpiresAt  time.Time
	RefreshHash      string
	RefreshExpiresAt time.Time
	IPAddress        *string
	UserAgent        *string
	DeviceID         *string
}

// LoginSessionResult reports what CreateLoginSession did.
type LoginSessionResult struct {
	FamilyUuid uuid.UUID
	// StaleFamilyUuid is the rotated or replaced family to invalidate (uuid.Nil if none).
	StaleFamilyUuid uuid.UUID
}

// UserProvider reads users for authentication; implemented by an adapter over the user feature.
type UserProvider interface {
	// GetCredentialsByEmail returns ErrUserNotFound when no user has that email.
	GetCredentialsByEmail(ctx context.Context, email string) (*Credentials, error)
}

// AuthRepository defines database operations for auth tokens.
type AuthRepository interface {
	// WithTransaction runs fn in a transaction.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	// CreateLoginSession rotates the device's session or creates a new one.
	CreateLoginSession(ctx context.Context, params NewSessionParams) (*LoginSessionResult, error)
	IssueTokenPair(ctx context.Context, params NewSessionParams) error
	GetSessionByHash(ctx context.Context, hash string) (*Session, error)
	GetTokenByHash(ctx context.Context, hash string) (*StoredToken, error)
	// ConsumeRefreshToken revokes a usable refresh token; false means it was not usable.
	ConsumeRefreshToken(ctx context.Context, hash string) (bool, error)
	RevokeFamily(ctx context.Context, familyUuid uuid.UUID, reason string) error
	// RevokeAllForUser revokes every active token of the user and returns the affected families.
	RevokeAllForUser(ctx context.Context, userUuid uuid.UUID, reason string) ([]uuid.UUID, error)
	ListActiveSessions(ctx context.Context, userID int64) ([]*SessionSummary, error)
	CleanupTokens(ctx context.Context, before time.Time) (int64, error)
}

// SessionCache caches sessions by token hash.
type SessionCache interface {
	// GetOrLoad returns the cached session for hash or calls load and caches the result.
	GetOrLoad(ctx context.Context, hash string, load func() (*Session, error)) (*Session, error)
	// InvalidateFamily makes every cached entry of the family stale.
	InvalidateFamily(ctx context.Context, familyUuid uuid.UUID)
}

// LoginGuard throttles failed logins per IP and per email.
type LoginGuard interface {
	// Check returns ErrIPBlocked or ErrTooManyAttempts when the IP or email is blocked.
	Check(ctx context.Context, ip, email string) error
	// RecordFailure counts a failed login and returns the IP's remaining attempts.
	RecordFailure(ctx context.Context, ip, email string) (remaining int64)
	// Succeeded clears the short-window counters after a successful login.
	Succeeded(ctx context.Context, ip, email string)
}

// AuthService defines business logic for authentication.
type AuthService interface {
	Login(ctx context.Context, input LoginInput) (*TokenPair, error)
	Refresh(ctx context.Context, refreshToken string, meta Metadata) (*TokenPair, error)
	Logout(ctx context.Context, identity *Identity) error
	LogoutAll(ctx context.Context, identity *Identity) error
	// AuthenticateAccess resolves a sealed access token; refresh tokens are rejected.
	AuthenticateAccess(ctx context.Context, token string) (*Identity, error)
	ListSessions(ctx context.Context, identity *Identity) ([]*SessionSummary, error)
	GetTokenDetails(ctx context.Context, token string) (*TokenDetails, error)
	RevokeToken(ctx context.Context, token string) error
	CleanupTokens(ctx context.Context, retention time.Duration) (int64, error)
}
