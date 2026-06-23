package store

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrSetupAlreadyComplete = errors.New("setup already complete")
	ErrSetupNotComplete     = errors.New("setup not complete")
)
