package constant

import "errors"

var (
	// Error messages
	ErrNotFound       = errors.New("cost not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrInternalServer = errors.New("internal server error")
)
