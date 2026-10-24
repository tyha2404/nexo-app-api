package constant

import (
	"errors"
)

var (
	// Error messages
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidInput        = errors.New("invalid input")
	ErrInternalServer      = errors.New("internal server error")
	ErrEmailAlreadyExists  = errors.New("email already exists")
	ErrUsernameTaken       = errors.New("username already taken")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInsufficientBalance = errors.New("insufficient balance in source wallet")
	ErrAlreadyExists       = errors.New("already exists")
	ErrSamePassword        = errors.New("new password must differ from the current one")
	// ErrPasswordManagedExternally: the account signs in through Supabase Auth
	// (the local password column is empty), so its password is changed there.
	ErrPasswordManagedExternally = errors.New("password is managed by Supabase Auth")
)
