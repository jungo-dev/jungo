package domain

import "github.com/jungo-dev/junkit/response"

// Domain errors for the auth feature.
var (
	ErrInvalidCredentials        = response.New(response.Unauthorized, "invalid_credentials")
	ErrInvalidCredentialsWarning = response.New(response.Unauthorized, "invalid_credentials_warning")
	ErrUserInactive              = response.New(response.Forbidden, "user_account_is_not_active")
	ErrTooManyAttempts           = response.New(response.TooManyRequests, "too_many_login_attempts")
	ErrIPBlocked                 = response.New(response.TooManyRequests, "ip_temporarily_blocked")
	ErrRecaptchaFailed           = response.New(response.Forbidden, "recaptcha_verification_failed")

	ErrInvalidToken     = response.New(response.Unauthorized, "invalid_token")
	ErrInvalidTokenType = response.New(response.Unauthorized, "invalid_token_type")
	ErrTokenExpired     = response.New(response.Unauthorized, "token_expired")
	ErrTokenRevoked     = response.New(response.Unauthorized, "token_revoked")
	ErrTokenReused      = response.New(response.Unauthorized, "token_reused")
	ErrTokenNotFound    = response.New(response.NotFound, "token_not_found")

	// ErrUserNotFound is returned by UserProvider; the service maps it to ErrInvalidCredentials.
	ErrUserNotFound = response.New(response.NotFound, "user_not_found")
)
