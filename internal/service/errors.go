package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email is already in use")
	ErrHandleTaken        = errors.New("handle is already in use")
	ErrInvalidHandle      = errors.New("handle must be 3-32 characters and use lowercase letters, numbers, or underscores")
	ErrInvalidEmail       = errors.New("email address is invalid")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
	ErrUserNotFound       = errors.New("user not found")
	ErrInactiveUser       = errors.New("user account is inactive")
	ErrLastAdmin          = errors.New("at least one active administrator is required")
	ErrCannotDeleteSelf   = errors.New("administrators cannot delete their own account")
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrInvitationUsed     = errors.New("invitation has already been used")
	ErrInvitationExpired  = errors.New("invitation has expired")
	ErrInvitationEmail      = errors.New("email address does not match invitation")
	ErrMailgunNotConfigured = errors.New("Mailgun settings have not been configured")
	ErrPasswordResetInvalid = errors.New("password reset link is invalid or has expired")
	ErrAccessRequestsDisabled = errors.New("public access requests are disabled")
	ErrGameNotFound             = errors.New("game not found")
	ErrInvalidGameName          = errors.New("name is required")
	ErrGameUpdateNotImplemented = errors.New("not yet implemented")
)
