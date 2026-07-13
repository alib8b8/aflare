package workflow

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkExpressionEvaluate(b *testing.B) {
	engine := NewExpressionEngine()
	engine.SetStepOutput(0, "fetch", `{"users":[{"name":"Alice"}]}`)
	engine.SetVariable("api_key", "secret123")
	engine.SetLoopVars("item1", 0, 3)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	exprs := []string{
		"{{input}}",
		"{{step.0}}",
		"{{var.api_key}}",
		"{{loop.item}}",
		"{{loop.index}}",
		"static text",
		"prefix-{{input}}-suffix",
		"{{step.0.jsonpath:$.users[0].name}}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			expr := exprs[rng.Intn(len(exprs))]
			input := fmt.Sprintf("test-%d", rng.Intn(10000))
			_, _ = engine.Evaluate(expr, input)
		}
	}
}
