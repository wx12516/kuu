package kuu

import (
	"errors"
)

var (
	ErrTokenNotFound  = errors.New("token not found")
	ErrSecretNotFound = errors.New("secret not found")
	ErrInvalidToken   = errors.New("invalid token")
)
