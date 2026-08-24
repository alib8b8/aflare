// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌‌‌‌‌‌‌​​​​​‌‌​‌​​​​‌​‌​​‌‌​​‌‌​​‌​‌‌‌‌​​​‌​​​​​​​​​​​​​​​​​​​​‌​‌‌​​‌‌​‌​​‌‌​⁠
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

package nodes

import (
	"context"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
)

const (
	maxRegexPatternLength = 512
	maxRegexInputLength   = 1024 * 1024 // 1MB
	maxRegexCacheSize     = 1000
	maxConcurrentRegex    = 100
)

var (
	regexCache     = make(map[string]*regexp.Regexp)
	regexCacheMu   sync.RWMutex
	regexSemaphore = make(chan struct{}, maxConcurrentRegex)
)

func compileRegexCached(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > maxRegexPatternLength {
		return nil, fmt.Errorf("regex pattern too long (max %d characters)", maxRegexPatternLength)
	}

	regexCacheMu.RLock()
	re, ok := regexCache[pattern]
	regexCacheMu.RUnlock()

	if ok {
		return re, nil
	}

	var err error
	re, err = regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	regexCacheMu.Lock()
	if len(regexCache) >= maxRegexCacheSize {
		for k := range regexCache {
			delete(regexCache, k)
			break
		}
	}
	regexCache[pattern] = re
	regexCacheMu.Unlock()

	return re, nil
}

func SafeRegexMatch(pattern, input string) (bool, error) {
	if len(pattern) > maxRegexPatternLength {
		return false, fmt.Errorf("regex pattern too long (max %d characters)", maxRegexPatternLength)
	}
	if len(input) > maxRegexInputLength {
		return false, fmt.Errorf("regex input too long (max %d bytes)", maxRegexInputLength)
	}

	re, err := compileRegexCached(pattern)
	if err != nil {
		return false, err
	}

	// Limit concurrent regex goroutines. The slot is released INSIDE the
	// goroutine (not via a caller defer) so it stays held until the match
	// truly terminates — even when the caller times out at the 2s select
	// below and returns. regexp has no cancellation API, so a ReDoS-prone
	// pattern can run indefinitely; releasing the slot on caller-timeout
	// would let new calls keep entering and leak goroutines without bound.
	// Holding the slot until goroutine exit bounds leaked goroutines to
	// maxConcurrentRegex (100); once exhausted, new calls fail fast with
	// "concurrency limit reached" instead of growing toward OOM.
	select {
	case regexSemaphore <- struct{}{}:
	case <-time.After(3 * time.Second):
		return false, fmt.Errorf("regex concurrency limit reached, try again later")
	}

	done := make(chan bool, 1)
	var matched bool
	go func() {
		defer func() {
			// Release the slot only when the match has actually finished
			// (success, panic, or runaway ReDoS finally returning).
			<-regexSemaphore
			if r := recover(); r != nil {
				logger.Error("regex match panicked",
					"pattern", pattern,
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
			// done is buffered (cap 1) so this send never blocks even if
			// the caller already timed out and returned.
			done <- true
		}()
		matched = re.MatchString(input)
	}()

	select {
	case <-done:
		return matched, nil
	case <-time.After(2 * time.Second):
		return false, fmt.Errorf("regex execution timed out")
	}
}

type ConditionNode struct{}

func init() {
	Register(&ConditionNode{})
}

func (n *ConditionNode) Name() string {
	return "condition"
}

func (n *ConditionNode) Description() string {
	return "Evaluate conditional expressions (contains, equals, regex, empty)"
}

func (n *ConditionNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "condition",
		Description: "Evaluate conditional expressions (contains, equals, regex, empty)",
		Input:       "string - the text to evaluate against",
		Output:      "string - 'true' or 'false'",
		Params: []ParamSchema{
			{Name: "expr", Type: "string", Description: "Condition expression (e.g. contains:foo, equals:bar, regex:^test, empty, not_empty)", Required: true},
			{Name: "condition", Type: "string", Description: "Alias for expr", Required: false},
		},
	}
}

// Execute evaluates a condition expression against the input.
// Supports simple patterns:
//   - "contains:keyword"  - true if input contains keyword
//   - "equals:value"      - true if input equals value
//   - "starts_with:prefix" - true if input starts with prefix
//   - "ends_with:suffix"   - true if input ends with suffix
//   - "regex:pattern"      - true if input matches regex
//   - "empty"              - true if input is empty
//   - "not_empty"          - true if input is not empty
//   - "true" / "false"     - literal
//
// When condition is true, returns "true"; when false, returns "false".
// Step-level conditional logic uses the skip_if/only_if metadata fields.
func (n *ConditionNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	expr, ok := params["expr"]
	if !ok || expr == "" {
		expr, ok = params["condition"]
		if !ok || expr == "" {
			return "", fmt.Errorf("expr or condition parameter is required")
		}
	}

	result, err := evaluateCondition(expr, input)
	if err != nil {
		return "", fmt.Errorf("condition evaluation failed: %w", err)
	}

	if result {
		return "true", nil
	}
	return "false", nil
}

func evaluateCondition(expr, input string) (bool, error) {
	// Trim whitespace
	expr = strings.TrimSpace(expr)
	input = strings.TrimSpace(input)

	// Check for "not_" prefix
	negate := false
	if strings.HasPrefix(expr, "not ") {
		negate = true
		expr = strings.TrimSpace(expr[4:])
	}

	result, err := evalPositive(expr, input)
	if err != nil {
		return false, err
	}

	if negate {
		return !result, nil
	}
	return result, nil
}

func evalPositive(expr, input string) (bool, error) {
	if expr == "empty" {
		return input == "", nil
	}
	if expr == "not_empty" {
		return input != "", nil
	}
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}

	colonIdx := strings.Index(expr, ":")
	if colonIdx < 0 {
		return false, fmt.Errorf("invalid condition format: %s", expr)
	}

	op := expr[:colonIdx]
	value := expr[colonIdx+1:]

	switch op {
	case "contains":
		return strings.Contains(input, value), nil
	case "equals":
		return input == value, nil
	case "starts_with":
		return strings.HasPrefix(input, value), nil
	case "ends_with":
		return strings.HasSuffix(input, value), nil
	case "regex":
		matched, err := SafeRegexMatch(value, input)
		if err != nil {
			return false, fmt.Errorf("regex evaluation failed: %w", err)
		}
		return matched, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", op)
	}
}
