package workflow

import (
	"runtime"
	"testing"
	"time"
)

func FuzzExpressionEvaluate(f *testing.F) {
	f.Add("{{input}}", "hello world")
	f.Add("{{step.0}}", "")
	f.Add("{{var.name}}", "")
	f.Add("static text", "anything")
	f.Add("{{env.PATH}}", "")
	f.Add("{{loop.item}}", "")
	f.Add("{{step.0.jsonpath:$.a}}", `{"a":1}`)
	f.Add("{{", "")
	f.Add("}}", "")
	f.Add("{{{{}}}}", "")
	f.Add("{{unknown.value}}", "test")
	f.Add("{{step.0.jsonpath:$..a}}", `{"a":{"a":1}}`)

	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"}]}`)
	engine.SetVariable("name", "test")
	engine.SetLoopVars("item1", 0, 3)

	f.Fuzz(func(t *testing.T, expr string, input string) {
		done := make(chan struct{})
		var panicErr interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = r
				}
				close(done)
			}()
			_, _ = engine.Evaluate(expr, input)
		}()

		select {
		case <-done:
			if panicErr != nil {
				t.Fatalf("Evaluate panicked: %v\nexpr=%q input=%q", panicErr, expr, input)
			}
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("Evaluate timed out\nexpr=%q input=%q\n%s", expr, input, buf[:n])
		}
	})
}
