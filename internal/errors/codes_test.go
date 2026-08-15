// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func TestNew_ErrorFormat(t *testing.T) {
	e := New(CodeNodeNotFound, "node 'llm' not found")
	want := "[NODE_NOT_FOUND] node 'llm' not found"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestNewf(t *testing.T) {
	e := Newf(CodeStepFailed, "step %s failed after %d retries", "fetch", 3)
	want := "[STEP_FAILED] step fetch failed after 3 retries"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestError_WithCauseAndStep(t *testing.T) {
	cause := stderrors.New("connection refused")
	e := &Error{Code: CodeLLMProviderFailed, Message: "OpenAI API failed", Step: "summarize", Cause: cause}
	want := "[LLM_PROVIDER_FAILED] step summarize: OpenAI API failed: connection refused"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
	if got := e.Unwrap(); !stderrors.Is(got, cause) {
		t.Errorf("Unwrap() = %v, want %v", got, cause)
	}
}

func TestError_StepWithoutCause(t *testing.T) {
	e := &Error{Code: CodeStepTimeout, Message: "deadline exceeded", Step: "transform"}
	want := "[STEP_TIMEOUT] step transform: deadline exceeded"
	if e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}

func TestWrap_NilCause(t *testing.T) {
	e := Wrap(nil, CodeInternal, "boom")
	if e.Cause != nil {
		t.Errorf("expected nil Cause, got %v", e.Cause)
	}
	if e.Code != CodeInternal {
		t.Errorf("Code = %q, want %q", e.Code, CodeInternal)
	}
}

func TestWrap_AvoidsDoubleWrapping(t *testing.T) {
	inner := New(CodePolicyDenied, "blocked by policy")
	outer := Wrap(inner, CodePolicyDenied, "blocked again")
	if outer != inner {
		t.Errorf("Wrap with same code should return inner as-is, got %v", outer)
	}
}

func TestWrapf_Chain(t *testing.T) {
	cause := stderrors.New("disk full")
	e := Wrapf(cause, CodeIdempotencyInternal, "store %s", "wal")
	if e.Code != CodeIdempotencyInternal {
		t.Errorf("Code = %q, want %q", e.Code, CodeIdempotencyInternal)
	}
	if !stderrors.Is(e, cause) {
		t.Error("errors.Is should reach the cause through the chain")
	}
}

func TestIs_SameCode(t *testing.T) {
	a := New(CodeWorkflowTimeout, "first")
	b := &Error{Code: CodeWorkflowTimeout, Message: "second"}
	if !stderrors.Is(a, b) {
		t.Error("errors.Is should match on Code regardless of Message")
	}
	if stderrors.Is(a, New(CodeInternal, "other")) {
		t.Error("errors.Is should not match different codes")
	}
}

func TestWithStep(t *testing.T) {
	if WithStep(nil, "s") != nil {
		t.Error("WithStep(nil) should return nil")
	}
	if WithStep(stderrors.New("plain"), "s") != nil {
		t.Error("WithStep(non-structured) should return nil")
	}

	orig := New(CodeStepFailed, "boom")
	st := WithStep(orig, "step-2")
	if st == nil || st.Step != "step-2" {
		t.Fatalf("WithStep returned %v, want Step=step-2", st)
	}
	if orig.Step != "" {
		t.Error("WithStep must not mutate the original error")
	}
}

func TestIsCode_GetCode_GetStep(t *testing.T) {
	e := WithStep(New(CodeExpressionEvalFailed, "bad expr"), "render")
	if !IsCode(e, CodeExpressionEvalFailed) {
		t.Error("IsCode should be true for the wrapped code")
	}
	if IsCode(e, CodeInternal) {
		t.Error("IsCode should be false for a different code")
	}
	if IsCode(stderrors.New("plain"), CodeInternal) {
		t.Error("IsCode should be false for a non-structured error")
	}
	if GetCode(e) != CodeExpressionEvalFailed {
		t.Errorf("GetCode = %q, want %q", GetCode(e), CodeExpressionEvalFailed)
	}
	if GetCode(stderrors.New("plain")) != "" {
		t.Error("GetCode should be empty for a non-structured error")
	}
	if GetStep(e) != "render" {
		t.Errorf("GetStep = %q, want %q", GetStep(e), "render")
	}
	if GetStep(New(CodeInternal, "x")) != "" {
		t.Error("GetStep should be empty when Step is unset")
	}
}

func TestGetCode_ThroughFmtWrap(t *testing.T) {
	inner := New(CodeWorkflowParseError, "bad yaml")
	outer := fmt.Errorf("run failed: %w", inner)
	if GetCode(outer) != CodeWorkflowParseError {
		t.Errorf("GetCode through fmt %%w = %q, want %q", GetCode(outer), CodeWorkflowParseError)
	}
	if !IsCode(outer, CodeWorkflowParseError) {
		t.Error("IsCode should see through fmt %w wrapping")
	}
}

func TestUnwrapAll(t *testing.T) {
	root := stderrors.New("root cause")
	middle := Wrap(root, CodeStepFailed, "middle")
	outer := Wrap(middle, CodeInternal, "outer")
	if got := UnwrapAll(outer); !stderrors.Is(got, root) {
		t.Errorf("UnwrapAll = %v, want %v", got, root)
	}
	plain := stderrors.New("plain")
	if got := UnwrapAll(plain); !stderrors.Is(got, plain) {
		t.Errorf("UnwrapAll on non-structured error should return it as-is, got %v", got)
	}
}
