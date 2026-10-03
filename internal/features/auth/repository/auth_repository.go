package repository

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/jungo-dev/junkit/database"

	"jungo/internal/database/sqlc"
	"jungo/internal/features/auth/domain"
	userdomain "jungo/internal/features/user/domain"
)

// AuthRepository implements domain.AuthRepository.
type AuthRepository struct {
	db *database.DB
}

// NewAuthRepository creates a AuthRepository backed by db.
func NewAuthRepository(db *database.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// WithTransaction implements domain.AuthRepository.
func (r *AuthRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.WithTransaction(ctx, fn)
}

// CreateLoginSession implements domain.AuthRepository via f_auth_login_session.
func (r *AuthRepository) CreateLoginSession(ctx context.Context, params domain.NewSessionParams) (*domain.LoginSessionResult, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).CreateLoginSession(ctx, sqlc.CreateLoginSessionParams{
		UserID:           params.UserID,
		DeviceID:         params.DeviceID,
		IpAddress:        parseIP(params.IPAddress),
		UserAgent:        params.UserAgent,
		AccessHash:       params.AccessHash,
		AccessExpiresAt:  params.AccessExpiresAt,
		RefreshHash:      params.RefreshHash,
		RefreshExpiresAt: params.RefreshExpiresAt,
		NewFamilyUuid:    params.FamilyUuid,
	})
	if err != nil {
		return nil, err
	}
	return &domain.LoginSessionResult{FamilyUuid: row.FamilyUuid, StaleFamilyUuid: row.OldFamilyUuid}, nil
}

// IssueTokenPair implements domain.AuthRepository.
func (r *AuthRepository) IssueTokenPair(ctx context.Context, params domain.NewSessionParams) error {
	return sqlc.New(r.db.Executor(ctx)).IssueTokenPair(ctx, sqlc.IssueTokenPairParams{
		UserID:           params.UserID,
		FamilyUuid:       params.FamilyUuid,
		AccessHash:       params.AccessHash,
		AccessExpiresAt:  params.AccessExpiresAt,
		RefreshHash:      params.RefreshHash,
		RefreshExpiresAt: params.RefreshExpiresAt,
		IpAddress:        parseIP(params.IPAddress),
		UserAgent:        params.UserAgent,
		DeviceID:         params.DeviceID,
	})
}

// GetSessionByHash implements domain.AuthRepository.
func (r *AuthRepository) GetSessionByHash(ctx context.Context, hash string) (*domain.Session, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).GetSessionByHash(ctx, hash)
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrTokenNotFound,
		})
	}
	return mapSession(row), nil
}

// GetTokenByHash implements domain.AuthRepository.
func (r *AuthRepository) GetTokenByHash(ctx context.Context, hash string) (*domain.StoredToken, error) {
	row, err := sqlc.New(r.db.Executor(ctx)).GetTokenByHash(ctx, hash)
	if err != nil {
		return nil, database.Match(err, map[database.ErrorType]error{
			database.ErrorNotFound: domain.ErrTokenNotFound,
		})
	}
	return &domain.StoredToken{
		Uuid:          row.Uuid,
		UserID:        row.UserID,
		UserUuid:      row.UserUuid,
		UserActive:    row.UserStatus == userdomain.UserStatusActive,
		FamilyUuid:    row.FamilyUuid,
		Type:          domain.TokenType(row.Type),
		DeviceID:      row.DeviceID,
		ExpiresAt:     row.ExpiresAt,
		RevokedAt:     row.RevokedAt,
		RevokedReason: row.RevokedReason,
	}, nil
}

// ConsumeRefreshToken implements domain.AuthRepository.
func (r *AuthRepository) ConsumeRefreshToken(ctx context.Context, hash string) (bool, error) {
	rowsAffected, err := sqlc.New(r.db.Executor(ctx)).ConsumeRefreshToken(ctx, hash)
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

// RevokeFamily implements domain.AuthRepository.
func (r *AuthRepository) RevokeFamily(ctx context.Context, familyUuid uuid.UUID, reason string) error {
	_, err := sqlc.New(r.db.Executor(ctx)).RevokeFamily(ctx, sqlc.RevokeFamilyParams{
		FamilyUuid: familyUuid,
		Reason:     &reason,
	})
	return err
}

// RevokeAllForUser implements domain.AuthRepository.
func (r *AuthRepository) RevokeAllForUser(ctx context.Context, userUuid uuid.UUID, reason string) ([]uuid.UUID, error) {
	families, err := sqlc.New(r.db.Executor(ctx)).RevokeAllForUser(ctx, sqlc.RevokeAllForUserParams{
		UserUuid: userUuid,
		Reason:   &reason,
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(families, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return slices.Compact(families), nil
}

// ListActiveSessions implements domain.AuthRepository.
func (r *AuthRepository) ListActiveSessions(ctx context.Context, userID int64) ([]*domain.SessionSummary, error) {
	rows, err := sqlc.New(r.db.Executor(ctx)).ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]*domain.SessionSummary, len(rows))
	for i, row := range rows {
		items[i] = &domain.SessionSummary{
			FamilyUuid: row.FamilyUuid,
			IPAddress:  formatIP(row.IpAddress),
			UserAgent:  row.UserAgent,
			DeviceID:   row.DeviceID,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			ExpiresAt:  row.ExpiresAt,
		}
	}
	return items, nil
}

// CleanupTokens implements domain.AuthRepository.
func (r *AuthRepository) CleanupTokens(ctx context.Context, before time.Time) (int64, error) {
	return sqlc.New(r.db.Executor(ctx)).CleanupTokens(ctx, before)
}

// mapSession converts a generated sqlc.GetSessionByHashRow to the domain model.
func mapSession(row sqlc.GetSessionByHashRow) *domain.Session {
	return &domain.Session{
		UserID:     row.UserID,
		UserUuid:   row.UserUuid,
		Email:      row.Email,
		FirstName:  row.FirstName,
		LastName:   row.LastName,
		AvatarUrl:  row.AvatarUrl,
		UserActive: row.UserStatus == userdomain.UserStatusActive,
		FamilyUuid: row.FamilyUuid,
		IPAddress:  formatIP(row.IpAddress),
		UserAgent:  row.UserAgent,
		DeviceID:   row.DeviceID,
		CreatedAt:  row.SessionCreatedAt,
		Access: domain.TokenState{
			Uuid:      row.AccessUuid,
			Hash:      row.AccessHash,
			ExpiresAt: row.AccessExpiresAt,
			RevokedAt: row.AccessRevokedAt,
		},
		Refresh: domain.TokenState{
			Uuid:      row.RefreshUuid,
			Hash:      row.RefreshHash,
			ExpiresAt: row.RefreshExpiresAt,
			RevokedAt: row.RefreshRevokedAt,
		},
	}
}

// parseIP converts a client IP string to the INET column type; unparsable input is stored as NULL.
func parseIP(ip *string) *netip.Addr {
	if ip == nil {
		return nil
	}
	addr, err := netip.ParseAddr(*ip)
	if err != nil {
		return nil
	}
	return &addr
}

// formatIP converts an INET column value back to a string.
func formatIP(addr *netip.Addr) *string {
	if addr == nil {
		return nil
	}
	s := addr.String()
	return &s
}
