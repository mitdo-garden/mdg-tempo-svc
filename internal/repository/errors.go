package repository

import "errors"

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrNotTempo = errors.New("not a tempo")
)
