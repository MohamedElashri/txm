package core

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"sort"
)

type OperationError struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Backend   *string         `json:"backend"`
	Locator   *SessionLocator `json:"locator"`
	Retryable bool            `json:"retryable"`
	Cause     error           `json:"-"`
}

func (e *OperationError) Error() string { return e.Message }
func (e *OperationError) Unwrap() error { return e.Cause }
func Failure(code, message string) *OperationError {
	return &OperationError{Code: code, Message: message, Retryable: code == "timeout" || code == "backend_unavailable"}
}
func Normalize(err error) *OperationError {
	var e *OperationError
	if errors.As(err, &e) {
		return e
	}
	code, message := "operation_failed", "operation failed"
	var n net.Error
	switch {
	case errors.Is(err, context.Canceled):
		code, message = "cancelled", "operation cancelled"
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &n) && n.Timeout()):
		code, message = "timeout", "operation timed out"
	case errors.Is(err, fs.ErrPermission):
		code, message = "denied", "permission denied"
	}
	e = Failure(code, message)
	e.Cause = err
	return e
}
func Scope(err error, backend string) *OperationError {
	e := *Normalize(err)
	e.Backend = String(backend)
	return &e
}
func ExitCode(errors []*OperationError, partial bool) int {
	for _, e := range errors {
		if e.Code == "cancelled" {
			return 130
		}
	}
	if partial {
		return 7
	}
	if len(errors) == 0 {
		return 0
	}
	// Ascending rank is the documented deterministic precedence.
	best, rank := 1, 99
	for _, e := range errors {
		code, r := 1, 8
		switch e.Code {
		case "cancelled":
			code, r = 130, -1
		case "invalid_usage", "invalid_target", "invalid_profile":
			code, r = 2, 0
		case "denied":
			code, r = 8, 1
		case "conflict", "ambiguous_target":
			code, r = 5, 2
		case "timeout":
			code, r = 6, 3
		case "backend_unavailable", "partial_failure":
			code, r = 7, 4
		case "unsupported", "unsupported_version":
			code, r = 4, 5
		case "not_found":
			code, r = 3, 6
		}
		if r < rank {
			best, rank = code, r
		}
	}
	return best
}
func SortErrors(es []*OperationError) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := "", ""
		if es[i].Backend != nil {
			a = *es[i].Backend
		}
		if es[j].Backend != nil {
			b = *es[j].Backend
		}
		if a != b {
			return a < b
		}
		return es[i].Code < es[j].Code
	})
}
