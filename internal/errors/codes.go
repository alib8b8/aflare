// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package errors provides a structured error taxonomy for aflare. It wraps
// the standard Go error interface with machine-readable error codes so that
// callers can programmatically classify and react to failures without parsing
// human-readable error strings.
//
// Usage:
//
//	// Creating a new error:
//	err := errors.New(errors.CodeNodeNotFound, "node 'llm' not found")
//
//	// Wrapping an existing error:
//	err := errors.Wrap(cause, errors.CodeLLMProviderFailed, "OpenAI API failed")
//
//	// Checking error codes:
//	if errors.IsCode(err, errors.CodePolicyDenied) { ... }
//
//	// Extracting the code:
//	code := errors.GetCode(err) // "" if not a structured error
//
// Error implements both error and Unwrap(), so it interoperates with
// errors.Is / errors.As and fmt.Errorf("...: %w", err).
package errors

import (
	"errors"
	"fmt"
)

// Code is a machine-readable error code following the UPPER_SNAKE convention.
// Callers use IsCode to match codes without string comparison.
type Code string

// ── Workflow lifecycle ──
const (
	// CodeWorkflowParseError indicates the workflow YAML or file could not be
	// parsed or read (syntax errors, missing files, permission problems).
	CodeWorkflowParseError Code = "WF_PARSE"

	// CodeWorkflowValidationError indicates the workflow definition is
	// structurally invalid (too many steps, invalid input schema, missing
	// required fields, circular dependencies).
	CodeWorkflowValidationError Code = "WF_VALIDATION"

	// CodeWorkflowTimeout indicates the overall workflow exceeded its deadline.
	CodeWorkflowTimeout Code = "WF_TIMEOUT"
)

// ── Step execution ──
const (
	// CodeNodeNotFound indicates a step referenced a node name that is not
	// registered in the node registry.
	CodeNodeNotFound Code = "NODE_NOT_FOUND"

	// CodeStepTimeout indicates a single step exceeded its per-step timeout.
	CodeStepTimeout Code = "STEP_TIMEOUT"

	// CodeStepFailed indicates a step execution failed (node returned an error
	// after exhausting retries and recovery primitives).
	CodeStepFailed Code = "STEP_FAILED"

	// CodeExpressionEvalFailed indicates a template expression could not be
	// evaluated (syntax errors, undefined variables, type mismatches).
	CodeExpressionEvalFailed Code = "EXPR_EVAL_FAILED"
)

// ── Policy & security ──
const (
	// CodePolicyDenied indicates a policy rule blocked the requested action
	// (filesystem, network, shell, or custom action).
	CodePolicyDenied Code = "POLICY_DENIED"

	// CodePolicyApprovalRequired indicates a policy requires human approval
	// before the action can proceed.
	CodePolicyApprovalRequired Code = "POLICY_APPROVAL_REQUIRED"

	// CodePolicyApprovalDenied indicates the human approver rejected the action.
	CodePolicyApprovalDenied Code = "POLICY_APPROVAL_DENIED"
)

// ── LLM / provider ──
const (
	// CodeLLMProviderFailed indicates the LLM provider API returned an error
	// (network errors, HTTP 4xx/5xx, rate limits, invalid responses).
	CodeLLMProviderFailed Code = "LLM_PROVIDER_FAILED"

	// CodeLLMAPIAuthError indicates the LLM provider API key is missing or
	// invalid (HTTP 401/403).
	CodeLLMAPIAuthError Code = "LLM_API_AUTH_ERROR"

	// CodeLLMAPIRateLimited indicates the LLM provider returned a rate-limit
	// response (HTTP 429).
	CodeLLMAPIRateLimited Code = "LLM_API_RATE_LIMITED"
)

// ── Idempotency ──
const (
	// CodeIdempotencyConflict indicates the workflow was skipped because the
	// idempotency key has already completed or is in progress.
	CodeIdempotencyConflict Code = "IDEMPOTENCY_CONFLICT"

	// CodeIdempotencyInternal indicates an internal error within the
	// idempotency store (I/O errors, corrupted records, lock timeouts).
	CodeIdempotencyInternal Code = "IDEMPOTENCY_INTERNAL"
)

// ── General ──
const (
	// CodeInternal indicates an unexpected internal error (panics recovered,
	// invariant violations, resource exhaustion).
	CodeInternal Code = "INTERNAL"
)

// Error is a structured error that carries a machine-readable Code, a
// human-readable Message, the optional Step name where the error occurred,
// and the underlying Cause (if wrapping another error).
//
// Error implements:
//   - error        (via Error() string)
//   - Unwrap()     (returns Cause, for errors.Is / errors.As)
//   - Is(target)   (so two Errors with the same Code compare equal)
type Error struct {
	Code    Code
	Message string
	Step    string
	Cause   error
}

// Error implements the error interface. The format is:
//
//	[CODE] message: cause
//
// When Step is non-empty, it is prepended:
//
//	[CODE] step <step>: message: cause
func (e *Error) Error() string {
	if e.Step != "" {
		if e.Cause != nil {
			return fmt.Sprintf("[%s] step %s: %s: %v", e.Code, e.Step, e.Message, e.Cause)
		}
		return fmt.Sprintf("[%s] step %s: %s", e.Code, e.Step, e.Message)
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying Cause, enabling errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is returns true when target is also an *Error with the same Code. This
// allows errors.Is(err, &Error{Code: CodeNodeNotFound}) to work without
// comparing the Message, Step, or Cause fields.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code
	}
	return false
}

// New creates a new Error with the given code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf creates a new Error with a formatted message.
func Newf(code Code, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap wraps an existing error with a code and message. If cause is nil, Wrap
// behaves like New. If cause is already an *Error with the same code, it is
// returned as-is to avoid double-wrapping.
func Wrap(cause error, code Code, message string) *Error {
	if cause == nil {
		return New(code, message)
	}
	// Avoid double-wrapping: if cause already has the same code, return it.
	var e *Error
	if errors.As(cause, &e) && e.Code == code {
		return e
	}
	return &Error{Code: code, Message: message, Cause: cause}
}

// Wrapf wraps an existing error with a code and formatted message.
func Wrapf(cause error, code Code, format string, args ...interface{}) *Error {
	return Wrap(cause, code, fmt.Sprintf(format, args...))
}

// WithStep returns a copy of the Error with the Step field set. If err is nil
// or not an *Error, it returns nil.
func WithStep(err error, step string) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		clone := *e
		clone.Step = step
		return &clone
	}
	return nil
}

// IsCode reports whether err (or any error in its chain) has the given code.
func IsCode(err error, code Code) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// GetCode returns the Code of the first *Error found in the error chain, or
// "" if the error is not a structured error.
func GetCode(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// GetStep returns the Step of the first *Error found in the error chain, or
// "" if not set.
func GetStep(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Step
	}
	return ""
}

// UnwrapAll returns the innermost Cause by repeatedly unwrapping until the
// error is no longer an *Error or has no Cause. This is useful for extracting
// the original Go error from a chain of wrapped structured errors.
func UnwrapAll(err error) error {
	for {
		var e *Error
		if !errors.As(err, &e) || e.Cause == nil {
			return err
		}
		err = e.Cause
	}
}