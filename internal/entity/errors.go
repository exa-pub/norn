package entity

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrInvalidName        = errors.New("invalid name")
	ErrFailedPrecondition = errors.New("failed precondition")
)
