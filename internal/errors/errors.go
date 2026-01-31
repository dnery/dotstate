// Package errors provides error types and exit codes for dotstate.
package errors

import (
	"errors"
	"fmt"
)

// Exit codes follow Unix conventions and sysexits.h patterns.
const (
	// ExitOK indicates successful execution.
	ExitOK = 0

	// ExitError indicates a general error.
	ExitError = 1

	// ExitUsage indicates incorrect command usage.
	ExitUsage = 2

	// ExitConfig indicates a configuration error.
	ExitConfig = 78

	// ExitUnavailable indicates a required service/resource is unavailable.
	ExitUnavailable = 69

	// ExitConflict indicates a merge conflict or similar.
	ExitConflict = 75

	// ExitPermission indicates a permission error.
	ExitPermission = 77

	// ExitCanceled indicates the operation was canceled.
	ExitCanceled = 130
)

// ExitError wraps an error with an exit code.
type ExitErr struct {
	Err  error
	Code int
}

func (e *ExitErr) Error() string {
	return e.Err.Error()
}

func (e *ExitErr) Unwrap() error {
	return e.Err
}

// Exit returns the exit code for an error.
// If the error is nil, returns ExitOK.
// If the error is an ExitErr, returns its code.
// Otherwise, returns ExitError.
func Exit(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitErr
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitError
}

// WithCode wraps an error with an exit code.
func WithCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return &ExitErr{Err: err, Code: code}
}

// ConfigError indicates a configuration-related error.
type ConfigError struct {
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("config error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("config error: %s", e.Message)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new configuration error.
func NewConfigError(msg string, err error) error {
	return &ExitErr{
		Err:  &ConfigError{Message: msg, Err: err},
		Code: ExitConfig,
	}
}

// ToolError indicates an external tool error.
type ToolError struct {
	Tool    string
	Message string
	Err     error
}

func (e *ToolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Tool, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Tool, e.Message)
}

func (e *ToolError) Unwrap() error {
	return e.Err
}

// NewToolError creates a new tool error.
func NewToolError(tool, msg string, err error) error {
	return &ExitErr{
		Err:  &ToolError{Tool: tool, Message: msg, Err: err},
		Code: ExitError,
	}
}

// ToolNotFoundError indicates a required tool is not installed.
type ToolNotFoundError struct {
	Tool        string
	InstallHint string
}

func (e *ToolNotFoundError) Error() string {
	if e.InstallHint != "" {
		return fmt.Sprintf("%s not found. Install with: %s", e.Tool, e.InstallHint)
	}
	return fmt.Sprintf("%s not found", e.Tool)
}

// NewToolNotFoundError creates a tool-not-found error.
func NewToolNotFoundError(tool, installHint string) error {
	return &ExitErr{
		Err:  &ToolNotFoundError{Tool: tool, InstallHint: installHint},
		Code: ExitUnavailable,
	}
}

// ConflictError indicates a merge conflict or similar issue.
type ConflictError struct {
	Message string
	Details string
}

func (e *ConflictError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("conflict: %s\n%s", e.Message, e.Details)
	}
	return fmt.Sprintf("conflict: %s", e.Message)
}

// NewConflictError creates a conflict error.
func NewConflictError(msg, details string) error {
	return &ExitErr{
		Err:  &ConflictError{Message: msg, Details: details},
		Code: ExitConflict,
	}
}

// UserError indicates a user-caused error (bad input, etc).
type UserError struct {
	Message string
}

func (e *UserError) Error() string {
	return e.Message
}

// NewUserError creates a user error.
func NewUserError(msg string) error {
	return &ExitErr{
		Err:  &UserError{Message: msg},
		Code: ExitUsage,
	}
}

// CanceledError indicates the operation was canceled by the user.
type CanceledError struct{}

func (e *CanceledError) Error() string {
	return "operation canceled"
}

// NewCanceledError creates a canceled error.
func NewCanceledError() error {
	return &ExitErr{
		Err:  &CanceledError{},
		Code: ExitCanceled,
	}
}

// Is reports whether any error in err's chain matches target.
// This is a convenience wrapper around errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
// This is a convenience wrapper around errors.As.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Wrap wraps an error with additional context.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf wraps an error with additional formatted context.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
