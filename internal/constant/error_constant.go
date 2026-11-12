package constant

import (
	"errors"
)

var (
	// Error messages
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidInput        = errors.New("invalid input")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUsernameTaken       = errors.New("username already taken")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInsufficientBalance = errors.New("insufficient balance in source wallet")
	ErrAlreadyExists       = errors.New("already exists")
	ErrSamePassword        = errors.New("new password must differ from the current one")
	// ErrPasswordManagedExternally: the account signs in through Supabase Auth
	// (the local password column is empty), so its password is changed there.
	ErrPasswordManagedExternally = errors.New("password is managed by Supabase Auth")
	// ErrInvalidRefreshToken: the refresh token is unknown, expired, revoked
	// or reused. Clients must sign in again.
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

// kindError is a sentinel error with its own message that also matches a
// generic kind (ErrInvalidInput, ErrNotFound, ...) through errors.Is. Callers
// that only know the kinds (e.g. the AI chat telling business errors from
// system errors) can classify it, while its message stays unchanged for the
// clients that display it.
type kindError struct {
	msg  string
	kind error
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }

// NewKindError returns a sentinel error with message msg that wraps kind.
// errors.Is(err, theSentinel) keeps working as for errors.New.
func NewKindError(kind error, msg string) error {
	return &kindError{msg: msg, kind: kind}
}
