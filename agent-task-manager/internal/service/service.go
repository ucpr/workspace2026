// Package service implements atama's core business logic — task and goal
// CRUD, dependency management, and Ready/next resolution — on top of
// package store (persistence) and package depgraph (graph algorithms). It
// is the single library shared by the CLI and, eventually, the TUI and
// GitHub sync (requirements §8.1).
package service

import (
	"fmt"

	"github.com/ucpr/atama/internal/store"
)

// Service is the entry point for all task/goal operations.
type Service struct {
	store *store.Store
}

// New wraps an initialized store.
func New(s *store.Store) *Service {
	return &Service{store: s}
}

// ValidationError reports a request that failed a business rule (as opposed
// to an I/O failure). CLI callers use this to pick an appropriate exit code.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func validationErrorf(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}
