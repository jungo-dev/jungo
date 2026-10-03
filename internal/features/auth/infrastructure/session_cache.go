package infrastructure

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jungo-dev/junkit/cache"

	"jungo/internal/config"
	"jungo/internal/features/auth/domain"
)

// cachedSession is a cached session and the time its DB read started.
type cachedSession struct {
	Session  *domain.Session `json:"session"`
	LoadedAt time.Time       `json:"loaded_at"`
}

// SessionCache implements domain.SessionCache. InvalidateFamily stores the invalidation
// time per family; entries loaded before it are treated as misses.
type SessionCache struct {
	sessions      cache.Cache[*cachedSession]
	invalidations cache.Cache[int64]
	ttl           time.Duration
	now           func() time.Time
}

// NewSessionCache creates a SessionCache using the app's cache driver.
func NewSessionCache(opts cache.Options, cfg *config.Config) *SessionCache {
	return &SessionCache{
		sessions:      cache.New[*cachedSession](opts),
		invalidations: cache.New[int64](opts),
		ttl:           cfg.Auth.SessionCacheTTL,
		now:           time.Now,
	}
}

// GetOrLoad implements domain.SessionCache; it returns a copy of the cached session.
func (c *SessionCache) GetOrLoad(ctx context.Context, hash string, load func() (*domain.Session, error)) (*domain.Session, error) {
	if c.ttl <= 0 {
		return load()
	}

	if entry, ok := c.sessions.Get(ctx, sessionKey(hash)); ok && entry != nil && entry.Session != nil && !c.stale(ctx, entry) {
		session := *entry.Session
		return &session, nil
	}

	// Taken before the read so a concurrent invalidation still marks it stale.
	loadedAt := c.now()
	session, err := load()
	if err != nil {
		return nil, err
	}

	_ = c.sessions.Set(ctx, sessionKey(hash), &cachedSession{Session: session, LoadedAt: loadedAt}, c.ttl)
	return session, nil
}

// InvalidateFamily implements domain.SessionCache.
func (c *SessionCache) InvalidateFamily(ctx context.Context, familyUuid uuid.UUID) {
	if c.ttl <= 0 || familyUuid == uuid.Nil {
		return
	}
	// 2x TTL: a slow load can write its entry after the marker.
	_ = c.invalidations.Set(ctx, familyKey(familyUuid), c.now().UnixNano(), 2*c.ttl)
}

// stale reports whether entry was loaded before its family was last invalidated.
func (c *SessionCache) stale(ctx context.Context, entry *cachedSession) bool {
	invalidatedAt, ok := c.invalidations.Get(ctx, familyKey(entry.Session.FamilyUuid))
	return ok && invalidatedAt >= entry.LoadedAt.UnixNano()
}

// sessionKey returns the cache key for the session of a token hash.
func sessionKey(hash string) string {
	return "auth:session:" + hash
}

// familyKey returns the cache key for a family's invalidation marker.
func familyKey(familyUuid uuid.UUID) string {
	return "auth:family:" + familyUuid.String() + ":invalidated_at"
}
