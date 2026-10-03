package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/jungo-dev/junkit/cache"
	"github.com/jungo-dev/junkit/security"
	"github.com/jungo-dev/junkit/telegram"

	"jungo/internal/config"
	"jungo/internal/features/auth/domain"
)

// notifyTimeout bounds the blacklist Telegram alert.
const notifyTimeout = 5 * time.Second

// LoginGuardParams defines dependencies for LoginGuard.
type LoginGuardParams struct {
	fx.In

	CacheOptions cache.Options
	Config       *config.Config
	Logger       *zap.Logger
	Telegram     telegram.Client `optional:"true"`
}

// LoginGuard implements domain.LoginGuard with fixed-window failure counters per IP
// (short window and blacklist window) and per email. Only failures are counted.
type LoginGuard struct {
	counters cache.Cache[int64]
	cfg      config.AuthConfig
	chatID   string
	env      string
	logger   *zap.Logger
	telegram telegram.Client
}

// NewLoginGuard creates a LoginGuard; it counts in memory when Redis is disabled.
func NewLoginGuard(p LoginGuardParams) *LoginGuard {
	opts := p.CacheOptions
	if opts.Driver != cache.DriverRedis {
		opts.Driver = cache.DriverMemory
	}
	return &LoginGuard{
		counters: cache.New[int64](opts),
		cfg:      p.Config.Auth,
		chatID:   p.Config.Telegram.ChatID,
		env:      p.Config.Environment,
		logger:   p.Logger,
		telegram: p.Telegram,
	}
}

// Check implements domain.LoginGuard.
func (g *LoginGuard) Check(ctx context.Context, ip, email string) error {
	if n, _ := g.counters.Get(ctx, ipLongKey(ip)); n >= g.cfg.LoginBlacklistAttempts {
		return domain.ErrIPBlocked
	}
	if n, _ := g.counters.Get(ctx, ipShortKey(ip)); n >= g.cfg.LoginMaxAttempts {
		return domain.ErrTooManyAttempts
	}
	if n, _ := g.counters.Get(ctx, emailKey(email)); n >= g.cfg.LoginEmailMaxAttempts {
		return domain.ErrTooManyAttempts
	}
	return nil
}

// RecordFailure implements domain.LoginGuard.
func (g *LoginGuard) RecordFailure(ctx context.Context, ip, email string) int64 {
	short := g.incr(ctx, ipShortKey(ip), g.cfg.LoginBlockWindow)
	long := g.incr(ctx, ipLongKey(ip), g.cfg.LoginBlacklistWindow)
	g.incr(ctx, emailKey(email), g.cfg.LoginBlockWindow)

	if long == g.cfg.LoginBlacklistAttempts {
		g.logger.Warn("login: IP blacklisted after repeated failures", zap.String("ip", ip), zap.Int64("failures", long))
		g.notifyBlacklisted(ctx, ip, long)
	}
	return max(g.cfg.LoginMaxAttempts-short, 0)
}

// Succeeded implements domain.LoginGuard. The blacklist counter is kept, so logging
// into an attacker-owned account cannot reset it.
func (g *LoginGuard) Succeeded(ctx context.Context, ip, email string) {
	_ = g.counters.Delete(ctx, ipShortKey(ip))
	_ = g.counters.Delete(ctx, emailKey(email))
}

// incr increments a counter; errors count as zero.
func (g *LoginGuard) incr(ctx context.Context, key string, ttl time.Duration) int64 {
	n, err := g.counters.Incr(ctx, key, ttl)
	if err != nil {
		g.logger.Error("login guard: increment counter", zap.String("key", key), zap.Error(err))
		return 0
	}
	return n
}

// notifyBlacklisted sends a best-effort Telegram alert when Telegram is configured.
func (g *LoginGuard) notifyBlacklisted(ctx context.Context, ip string, failures int64) {
	if g.telegram == nil || g.chatID == "" {
		return
	}

	msg := fmt.Sprintf("🛡️ <b>Security Alert: Brute Force Attempt</b>\n"+
		"<b>Environment:</b> %s\n"+
		"<b>IP Address:</b> <code>%s</code>\n"+
		"<b>Time:</b> %s\n"+
		"⛔ <b>Action:</b> IP blocked for <b>%s</b> (%d failed attempts).",
		g.env, ip, time.Now().Format(time.DateTime), g.cfg.LoginBlacklistWindow, failures)

	go func() {
		sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifyTimeout)
		defer cancel()
		if err := g.telegram.SendMessage(sendCtx, g.chatID, msg); err != nil {
			g.logger.Error("login guard: telegram alert", zap.Error(err))
		}
	}()
}

// ipShortKey returns the short-window counter key for ip.
func ipShortKey(ip string) string {
	return "auth:login:ip:short:" + ip
}

// ipLongKey returns the long-window counter key for ip.
func ipLongKey(ip string) string {
	return "auth:login:ip:long:" + ip
}

// emailKey returns the counter key for email (hashed, so addresses are not stored).
func emailKey(email string) string {
	return "auth:login:email:" + security.HashToken(strings.ToLower(strings.TrimSpace(email)))
}
