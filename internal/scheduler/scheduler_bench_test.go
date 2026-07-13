package scheduler

import (
	"testing"
	"time"
)

func BenchmarkNextRun(b *testing.B) {
	schedule, err := parseCronExpr("* * * * *")
	if err != nil {
		b.Fatalf("failed to parse cron: %v", err)
	}
	from := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			nextRunTime(schedule, from)
		}
	}
}
