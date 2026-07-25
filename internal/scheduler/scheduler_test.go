// Copyright (c) 2026 llm-box Contributors
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

package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseCronExpr_AllWildcards(t *testing.T) {
	schedule, err := parseCronExpr("* * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 60 {
		t.Errorf("expected 60 minutes, got %d", len(schedule.minute))
	}
	if len(schedule.hour) != 24 {
		t.Errorf("expected 24 hours, got %d", len(schedule.hour))
	}
	if len(schedule.dayOfMonth) != 31 {
		t.Errorf("expected 31 days, got %d", len(schedule.dayOfMonth))
	}
	if len(schedule.month) != 12 {
		t.Errorf("expected 12 months, got %d", len(schedule.month))
	}
	if len(schedule.dayOfWeek) != 7 {
		t.Errorf("expected 7 days of week, got %d", len(schedule.dayOfWeek))
	}
}

func TestParseCronExpr_SingleValue(t *testing.T) {
	schedule, err := parseCronExpr("30 14 15 6 3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 1 || schedule.minute[0] != 30 {
		t.Errorf("expected minute 30, got %v", schedule.minute)
	}
	if len(schedule.hour) != 1 || schedule.hour[0] != 14 {
		t.Errorf("expected hour 14, got %v", schedule.hour)
	}
	if len(schedule.dayOfMonth) != 1 || schedule.dayOfMonth[0] != 15 {
		t.Errorf("expected day 15, got %v", schedule.dayOfMonth)
	}
	if len(schedule.month) != 1 || schedule.month[0] != 6 {
		t.Errorf("expected month 6, got %v", schedule.month)
	}
	if len(schedule.dayOfWeek) != 1 || schedule.dayOfWeek[0] != 3 {
		t.Errorf("expected weekday 3, got %v", schedule.dayOfWeek)
	}
}

func TestParseCronExpr_List(t *testing.T) {
	schedule, err := parseCronExpr("1,3,5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 3 {
		t.Fatalf("expected 3 values, got %d", len(schedule.minute))
	}
	expected := []int{1, 3, 5}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_Range(t *testing.T) {
	schedule, err := parseCronExpr("1-5 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 5 {
		t.Fatalf("expected 5 values, got %d", len(schedule.minute))
	}
	for i := 0; i < 5; i++ {
		if schedule.minute[i] != i+1 {
			t.Errorf("expected %d at index %d, got %d", i+1, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_Step(t *testing.T) {
	schedule, err := parseCronExpr("*/15 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 4 {
		t.Fatalf("expected 4 values, got %d", len(schedule.minute))
	}
	expected := []int{0, 15, 30, 45}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_RangeWithStep(t *testing.T) {
	schedule, err := parseCronExpr("1-10/2 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schedule.minute) != 5 {
		t.Fatalf("expected 5 values, got %d", len(schedule.minute))
	}
	expected := []int{1, 3, 5, 7, 9}
	for i, v := range expected {
		if schedule.minute[i] != v {
			t.Errorf("expected %d at index %d, got %d", v, i, schedule.minute[i])
		}
	}
}

func TestParseCronExpr_InvalidFieldCount(t *testing.T) {
	_, err := parseCronExpr("* * * *")
	if err == nil {
		t.Error("expected error for 4 fields")
	}

	_, err = parseCronExpr("* * * * * *")
	if err == nil {
		t.Error("expected error for 6 fields")
	}
}

func TestParseCronExpr_InvalidValue(t *testing.T) {
	_, err := parseCronExpr("60 * * * *")
	if err == nil {
		t.Error("expected error for minute 60")
	}

	_, err = parseCronExpr("* 24 * * *")
	if err == nil {
		t.Error("expected error for hour 24")
	}

	_, err = parseCronExpr("* * 32 * *")
	if err == nil {
		t.Error("expected error for day 32")
	}

	_, err = parseCronExpr("* * * 13 *")
	if err == nil {
		t.Error("expected error for month 13")
	}

	_, err = parseCronExpr("* * * * 7")
	if err == nil {
		t.Error("expected error for weekday 7")
	}
}

func TestNextRunTime_EveryMinute(t *testing.T) {
	schedule, _ := parseCronExpr("* * * * *")
	from := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_SpecificMinute(t *testing.T) {
	schedule, _ := parseCronExpr("30 * * * *")
	from := time.Date(2025, 1, 15, 10, 15, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_NextHour(t *testing.T) {
	schedule, _ := parseCronExpr("30 * * * *")
	from := time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestNextRunTime_NextDay(t *testing.T) {
	schedule, _ := parseCronExpr("0 0 * * *")
	from := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	next := nextRunTime(schedule, from)

	expected := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, next)
	}
}

func TestScheduler_AddRemoveTask(t *testing.T) {
	s := New()

	err := s.AddTask("test1", "* * * * *", func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.AddTask("test1", "* * * * *", func() {})
	if err == nil {
		t.Error("expected error for duplicate task")
	}

	err = s.RemoveTask("test1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = s.RemoveTask("test1")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestScheduler_AddTask_InvalidExpr(t *testing.T) {
	s := New()
	err := s.AddTask("bad", "invalid expr", func() {})
	if err == nil {
		t.Error("expected error for invalid expression")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()

	var count int64
	s.AddTask("test", "* * * * *", func() {
		atomic.AddInt64(&count, 1)
	})

	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()
}

func TestScheduler_StartTwice(t *testing.T) {
	s := New()
	s.Start()
	s.Start()
	s.Stop()
}

func TestScheduler_StopTwice(t *testing.T) {
	s := New()
	s.Start()
	s.Stop()
	s.Stop()
}

func TestScheduler_ThreadSafety(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = s.AddTask(string(rune('a'+i%26))+string(rune('0'+i/26)), "* * * * *", func() {})
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = s.RemoveTask(string(rune('a'+i%26)) + string(rune('0'+i/26)))
		}(i)
	}
	wg.Wait()
}

func TestContains(t *testing.T) {
	slice := []int{1, 3, 5, 7, 9}

	if !contains(slice, 3) {
		t.Error("expected contains 3")
	}
	if contains(slice, 4) {
		t.Error("expected not contains 4")
	}
}

func TestDaysInMonth(t *testing.T) {
	if daysInMonth(2025, time.February) != 28 {
		t.Error("expected 28 days in Feb 2025")
	}
	if daysInMonth(2024, time.February) != 29 {
		t.Error("expected 29 days in Feb 2024")
	}
	if daysInMonth(2025, time.January) != 31 {
		t.Error("expected 31 days in January")
	}
	if daysInMonth(2025, time.April) != 30 {
		t.Error("expected 30 days in April")
	}
}
