package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/jungo-dev/junkit/recaptcha"
	"github.com/jungo-dev/junkit/security"
	"github.com/jungo-dev/junkit/tracer"

	"jungo/internal/config"
	"jungo/internal/features/auth/domain"
)

// Client metadata limits; longer values are truncated.
const (
	maxDeviceIDLength  = 100
	maxUserAgentLength = 512
)

// warnBelowRemaining is the remaining-attempts count at which failed logins get a warning.
const warnBelowRemaining = 2

// errRefreshReused aborts the refresh transaction when the token was already consumed.
var errRefreshReused = errors.New("refresh token reused")

// Params defines dependencies for AuthService.
type Params struct {
	fx.In

	Config    *config.Config
	Repo      domain.AuthRepository
	Cache     domain.SessionCache
	Guard     domain.LoginGuard
	Users     domain.UserProvider
	Sealer    security.TokenSealer
	Hasher    security.PasswordHasher
	Logger    *zap.Logger
	Recaptcha recaptcha.Verifier `optional:"true"`
}

// AuthService implements domain.AuthService.
type AuthService struct {
	cfg       config.AuthConfig
	repo      domain.AuthRepository
	cache     domain.SessionCache
	guard     domain.LoginGuard
	users     domain.UserProvider
	sealer    security.TokenSealer
	hasher    security.PasswordHasher
	logger    *zap.Logger
	recaptcha recaptcha.Verifier
	now       func() time.Time
}

// NewAuthService creates a AuthService.
func NewAuthService(p Params) *AuthService {
	return &AuthService{
		cfg:       p.Config.Auth,
		repo:      p.Repo,
		cache:     p.Cache,
		guard:     p.Guard,
		users:     p.Users,
		sealer:    p.Sealer,
		hasher:    p.Hasher,
		logger:    p.Logger,
		recaptcha: p.Recaptcha,
		now:       time.Now,
	}
}

// Login authenticates email/password and issues a token pair.
//
// Flow: recaptcha -> brute-force check -> verify password -> check status -> create/rotate session
func (s *AuthService) Login(ctx context.Context, input domain.LoginInput) (*domain.TokenPair, error) {
	meta := input.Metadata
	email := strings.TrimSpace(input.Email)

	if s.cfg.RecaptchaOnLogin && s.recaptcha != nil {
		if err := s.recaptcha.Verify(ctx, meta.RecaptchaToken, meta.IPAddress); err != nil {
			return nil, domain.ErrRecaptchaFailed
		}
	}

	if err := s.guard.Check(ctx, meta.IPAddress, email); err != nil {
		return nil, err
	}

	// Unknown emails still run bcrypt so timing does not reveal registered emails.
	creds, err := s.users.GetCredentialsByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	stop := tracer.Span(ctx, "Bcrypt Compare")
	var valid bool
	if creds == nil {
		s.hasher.CompareDummy(input.Password)
	} else {
		valid = s.hasher.Compare(creds.PasswordHash, input.Password)
	}
	stop()

	if !valid {
		if remaining := s.guard.RecordFailure(ctx, meta.IPAddress, email); remaining <= warnBelowRemaining {
			return nil, domain.ErrInvalidCredentialsWarning
		}
		return nil, domain.ErrInvalidCredentials
	}

	// Status is checked after the password so it is not disclosed to guessers.
	if !creds.Active {
		return nil, domain.ErrUserInactive
	}

	pair, params, err := s.newTokenPair(creds.UserID, uuid.New(), meta)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.CreateLoginSession(ctx, params)
	if err != nil {
		return nil, err
	}
	s.cache.InvalidateFamily(ctx, result.StaleFamilyUuid)

	s.guard.Succeeded(ctx, meta.IPAddress, email)
	return pair, nil
}

// Refresh exchanges a refresh token for a new pair in the same session.
//
// Flow: open token -> checks -> [tx: consume -> revoke family -> issue pair] -> invalidate cache
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, meta domain.Metadata) (*domain.TokenPair, error) {
	hash, err := s.openToken(refreshToken)
	if err != nil {
		return nil, err
	}

	token, err := s.repo.GetTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}

	switch {
	case token.Type != domain.TokenTypeRefresh:
		return nil, domain.ErrInvalidTokenType
	case token.RevokedAt != nil && token.RevokedReason != nil && *token.RevokedReason == domain.RevokedReasonRefresh:
		return nil, s.handleRefreshReuse(ctx, token)
	case token.RevokedAt != nil:
		return nil, domain.ErrTokenRevoked
	case !s.now().Before(token.ExpiresAt):
		return nil, domain.ErrTokenExpired
	case !token.UserActive:
		return nil, domain.ErrTokenRevoked
	}

	// Keep the session's device when the client omits X-Device-ID.
	if strings.TrimSpace(meta.DeviceID) == "" && token.DeviceID != nil {
		meta.DeviceID = *token.DeviceID
	}

	pair, params, err := s.newTokenPair(token.UserID, token.FamilyUuid, meta)
	if err != nil {
		return nil, err
	}
	err = s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		consumed, err := s.repo.ConsumeRefreshToken(txCtx, hash)
		if err != nil {
			return err
		}
		if !consumed {
			return errRefreshReused
		}
		if err := s.repo.RevokeFamily(txCtx, token.FamilyUuid, domain.RevokedReasonRefresh); err != nil {
			return err
		}
		return s.repo.IssueTokenPair(txCtx, params)
	})
	if errors.Is(err, errRefreshReused) {
		return nil, s.handleRefreshReuse(ctx, token)
	}
	if err != nil {
		return nil, err
	}

	s.cache.InvalidateFamily(ctx, token.FamilyUuid)
	return pair, nil
}

// Logout ends the session the identity's access token belongs to.
func (s *AuthService) Logout(ctx context.Context, identity *domain.Identity) error {
	if err := s.repo.RevokeFamily(ctx, identity.FamilyUuid, domain.RevokedReasonLogout); err != nil {
		return err
	}
	s.cache.InvalidateFamily(ctx, identity.FamilyUuid)
	return nil
}

// LogoutAll ends every session of the identity's user.
func (s *AuthService) LogoutAll(ctx context.Context, identity *domain.Identity) error {
	families, err := s.repo.RevokeAllForUser(ctx, identity.UserUuid, domain.RevokedReasonLogout)
	if err != nil {
		return err
	}
	for _, family := range families {
		s.cache.InvalidateFamily(ctx, family)
	}
	return nil
}

// AuthenticateAccess resolves a sealed access token to the caller's identity.
//
// Flow: open token -> session (cache, then DB) -> match access hash -> check expiry/revocation
func (s *AuthService) AuthenticateAccess(ctx context.Context, token string) (*domain.Identity, error) {
	hash, err := s.openToken(token)
	if err != nil {
		return nil, err
	}

	session, err := s.getSession(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return nil, domain.ErrInvalidToken
		}
		return nil, err
	}

	switch {
	case hash == session.Refresh.Hash:
		return nil, domain.ErrInvalidTokenType
	case hash != session.Access.Hash:
		// Superseded by a refresh or re-login.
		return nil, domain.ErrTokenRevoked
	case !session.UserActive:
		return nil, domain.ErrTokenRevoked
	}

	switch session.Access.Status(s.now()) {
	case domain.TokenStatusRevoked:
		return nil, domain.ErrTokenRevoked
	case domain.TokenStatusExpired:
		return nil, domain.ErrTokenExpired
	}

	return &domain.Identity{
		UserID:     session.UserID,
		UserUuid:   session.UserUuid,
		Email:      session.Email,
		FamilyUuid: session.FamilyUuid,
		TokenUuid:  session.Access.Uuid,
		ExpiresAt:  session.Access.ExpiresAt,
		Session:    session,
	}, nil
}

// ListSessions returns the user's active sessions, flagging the caller's own.
func (s *AuthService) ListSessions(ctx context.Context, identity *domain.Identity) ([]*domain.SessionSummary, error) {
	sessions, err := s.repo.ListActiveSessions(ctx, identity.UserID)
	if err != nil {
		return nil, err
	}
	for _, item := range sessions {
		item.Current = item.FamilyUuid == identity.FamilyUuid
	}
	return sessions, nil
}

// GetTokenDetails describes any sealed token, including expired and revoked ones (internal use).
func (s *AuthService) GetTokenDetails(ctx context.Context, token string) (*domain.TokenDetails, error) {
	hash, err := s.openToken(token)
	if err != nil {
		return nil, err
	}

	session, err := s.repo.GetSessionByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	details := &domain.TokenDetails{Session: session}
	switch hash {
	case session.Access.Hash:
		details.TokenType, details.Status = domain.TokenTypeAccess, session.Access.Status(s.now())
	case session.Refresh.Hash:
		details.TokenType, details.Status = domain.TokenTypeRefresh, session.Refresh.Status(s.now())
	default:
		stored, err := s.repo.GetTokenByHash(ctx, hash)
		if err != nil {
			return nil, err
		}
		details.TokenType, details.Status = stored.Type, domain.TokenStatusRevoked
	}
	return details, nil
}

// RevokeToken ends the session a sealed token belongs to (internal use).
func (s *AuthService) RevokeToken(ctx context.Context, token string) error {
	hash, err := s.openToken(token)
	if err != nil {
		return err
	}

	stored, err := s.repo.GetTokenByHash(ctx, hash)
	if err != nil {
		return err
	}
	if err := s.repo.RevokeFamily(ctx, stored.FamilyUuid, domain.RevokedReasonAdmin); err != nil {
		return err
	}
	s.cache.InvalidateFamily(ctx, stored.FamilyUuid)
	return nil
}

// CleanupTokens deletes tokens that expired or were revoked more than retention ago.
func (s *AuthService) CleanupTokens(ctx context.Context, retention time.Duration) (int64, error) {
	return s.repo.CleanupTokens(ctx, s.now().Add(-retention))
}

// openToken unseals a client token and returns its stored hash; raw tokens and hashes are rejected.
func (s *AuthService) openToken(token string) (string, error) {
	raw, err := s.sealer.Open(token)
	if err != nil {
		return "", domain.ErrInvalidToken
	}
	return security.HashToken(raw), nil
}

// getSession loads the session for hash through the cache.
func (s *AuthService) getSession(ctx context.Context, hash string) (*domain.Session, error) {
	return s.cache.GetOrLoad(ctx, hash, func() (*domain.Session, error) {
		return s.repo.GetSessionByHash(ctx, hash)
	})
}

// handleRefreshReuse revokes the whole family when a used refresh token is presented again.
func (s *AuthService) handleRefreshReuse(ctx context.Context, token *domain.StoredToken) error {
	s.logger.Warn("auth: refresh token reuse detected, revoking session",
		zap.String("family_uuid", token.FamilyUuid.String()),
		zap.String("user_uuid", token.UserUuid.String()),
	)
	if err := s.repo.RevokeFamily(ctx, token.FamilyUuid, domain.RevokedReasonReuse); err != nil {
		return err
	}
	s.cache.InvalidateFamily(ctx, token.FamilyUuid)
	return domain.ErrTokenReused
}

// newTokenPair generates a sealed token pair and the hashed values to persist.
func (s *AuthService) newTokenPair(userID int64, familyUuid uuid.UUID, meta domain.Metadata) (*domain.TokenPair, domain.NewSessionParams, error) {
	now := s.now()
	accessRaw, refreshRaw := security.GenerateToken(), security.GenerateToken()

	params := domain.NewSessionParams{
		UserID:           userID,
		FamilyUuid:       familyUuid,
		AccessHash:       security.HashToken(accessRaw),
		AccessExpiresAt:  now.Add(s.cfg.AccessTokenTTL),
		RefreshHash:      security.HashToken(refreshRaw),
		RefreshExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
		IPAddress:        optional(meta.IPAddress, 0),
		UserAgent:        optional(meta.UserAgent, maxUserAgentLength),
		DeviceID:         optional(meta.DeviceID, maxDeviceIDLength),
	}

	accessToken, err := s.sealer.Seal(accessRaw)
	if err != nil {
		return nil, params, err
	}
	refreshToken, err := s.sealer.Seal(refreshRaw)
	if err != nil {
		return nil, params, err
	}

	return &domain.TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  params.AccessExpiresAt,
		RefreshExpiresAt: params.RefreshExpiresAt,
	}, params, nil
}

// optional trims and truncates v (maxLen 0 = unlimited); empty becomes nil.
func optional(v string, maxLen int) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if runes := []rune(v); maxLen > 0 && len(runes) > maxLen {
		v = string(runes[:maxLen])
	}
	return &v
}
