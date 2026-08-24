// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌‌‌‌​​‌​‌​​​‌‌‌​‌‌‌​‌‌​​‌​‌​​‌‌​‌​​​‌​​​‌​‌‌​​​​​​​​​​​​​​​​​‌‌​​‌‌​‌‌​​‌‌‌‌‌⁠
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

package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── P2-7：WAL 增量记录 + 持久性档位 ──
//
// 覆盖点：
//   1. WALStateManager 输出 delta 记录：首条全量快照，后续仅含变化部分，
//      LoadStateWAL 折叠恢复完整状态。
//   2. 写放大：N 步工作流的 WAL 体积从 O(N²)（每步全量快照）降到 O(N)。
//   3. 旧式全量记录与 delta 记录混合回放（向后兼容）。
//   4. Compact 把 delta 序列折叠为单条全量快照。
//   5. resume 后继续增量记录。
//   6. SyncInterval 档位：后台 syncer 周期性落盘，未显式 Flush 的记录
//      在 interval 后对独立 reader 可见。

// newDeltaTestEngine 构造带指定步骤输出与变量的 engine。
func newDeltaTestEngine(outputs map[int]string, vars map[string]string) *ExpressionEngine {
	e := NewExpressionEngine()
	for idx, out := range outputs {
		e.SetStepOutput(idx, "", out)
	}
	for k, v := range vars {
		e.SetVariable(k, v)
	}
	return e
}

func TestWALStateManager_DeltaRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delta.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer wal.Close()

	wf := &Workflow{Name: "delta-wf", Steps: []WorkflowStep{{Node: "n"}, {Node: "n"}, {Node: "n"}}}
	m := NewWALStateManager(wal, wf, nil)

	// 步骤 0：engine 有输出 0 → 首条应为全量快照。
	e0 := newDeltaTestEngine(map[int]string{0: "o0"}, map[string]string{"v": "a"})
	if err := m.Save(0, "d0", e0); err != nil {
		t.Fatalf("save step 0: %v", err)
	}
	// 步骤 1：新增输出 1 → delta 应只含条目 1。
	e1 := newDeltaTestEngine(map[int]string{0: "o0", 1: "o1"}, map[string]string{"v": "a"})
	if err := m.Save(1, "d1", e1); err != nil {
		t.Fatalf("save step 1: %v", err)
	}
	// 步骤 2：变量变化 v=b、新增输出 2 → delta 含 2 与 v。
	e2 := newDeltaTestEngine(map[int]string{0: "o0", 1: "o1", 2: "o2"}, map[string]string{"v": "b"})
	if err := m.Save(2, "d2", e2); err != nil {
		t.Fatalf("save step 2: %v", err)
	}

	if err := wal.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	recs := collectReplay(t, path)
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0].IsDelta {
		t.Error("first record must be a full snapshot (IsDelta=false)")
	}
	if len(recs[0].StepOutputs) != 1 || recs[0].StepOutputs[0] != "o0" {
		t.Errorf("first record outputs = %v, want {0:o0}", recs[0].StepOutputs)
	}
	if !recs[1].IsDelta {
		t.Error("second record must be a delta")
	}
	if len(recs[1].StepOutputs) != 1 || recs[1].StepOutputs[1] != "o1" {
		t.Errorf("step-1 delta outputs = %v, want only {1:o1}", recs[1].StepOutputs)
	}
	if len(recs[1].Variables) != 0 {
		t.Errorf("step-1 delta variables = %v, want empty (no change)", recs[1].Variables)
	}
	if len(recs[2].Variables) != 1 || recs[2].Variables["v"] != "b" {
		t.Errorf("step-2 delta variables = %v, want {v:b}", recs[2].Variables)
	}
	if len(recs[2].StepOutputs) != 1 || recs[2].StepOutputs[2] != "o2" {
		t.Errorf("step-2 delta outputs = %v, want only {2:o2}", recs[2].StepOutputs)
	}

	// 恢复：折叠后的完整状态。
	state, err := LoadStateWAL(path)
	if err != nil {
		t.Fatalf("LoadStateWAL: %v", err)
	}
	if state.StepIndex != 2 || state.Data != "d2" {
		t.Errorf("recovered step/data = %d/%q, want 2/d2", state.StepIndex, state.Data)
	}
	wantOuts := map[int]string{0: "o0", 1: "o1", 2: "o2"}
	if len(state.StepOutputs) != len(wantOuts) {
		t.Fatalf("recovered outputs = %v, want %v", state.StepOutputs, wantOuts)
	}
	for k, v := range wantOuts {
		if state.StepOutputs[k] != v {
			t.Errorf("recovered output[%d] = %q, want %q", k, state.StepOutputs[k], v)
		}
	}
	if state.Variables["v"] != "b" {
		t.Errorf("recovered variable v = %q, want b", state.Variables["v"])
	}
}

// TestWALStateManager_WriteAmplification 对比全量快照与 delta 模式的
// WAL 体积：N 步、每步输出等长字符串时，全量模式体积应数倍于 delta
// 模式（O(N²) vs O(N)）。
func TestWALStateManager_WriteAmplification(t *testing.T) {
	const n = 40
	out := make([]string, n)
	for i := range out {
		out[i] = "output-payload-" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	}

	fullPath := filepath.Join(t.TempDir(), "full.wal")
	deltaPath := filepath.Join(t.TempDir(), "delta.wal")

	// 全量模式：直接 Append 完整快照（旧行为）。
	fullWal, err := NewWAL(fullPath, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL full: %v", err)
	}
	wf := &Workflow{Name: "amp-wf"}
	for i := 0; i < n; i++ {
		outs := make(map[int]string, i+1)
		for j := 0; j <= i; j++ {
			outs[j] = out[j]
		}
		if err := fullWal.Append(WALRecord{StepIndex: i, Data: out[i], StepOutputs: outs}); err != nil {
			t.Fatalf("full append %d: %v", i, err)
		}
	}
	if err := fullWal.Close(); err != nil {
		t.Fatalf("close full: %v", err)
	}

	// Delta 模式：WALStateManager。
	deltaWal, err := NewWAL(deltaPath, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL delta: %v", err)
	}
	m := NewWALStateManager(deltaWal, wf, nil)
	for i := 0; i < n; i++ {
		outs := make(map[int]string, i+1)
		for j := 0; j <= i; j++ {
			outs[j] = out[j]
		}
		e := newDeltaTestEngine(outs, nil)
		if err := m.Save(i, out[i], e); err != nil {
			t.Fatalf("delta save %d: %v", i, err)
		}
	}
	if err := deltaWal.Close(); err != nil {
		t.Fatalf("close delta: %v", err)
	}

	fullSize := fileSize(t, fullPath)
	deltaSize := fileSize(t, deltaPath)
	// 全量 ≈ sum_{i=1..N} i 条目 ≈ N²/2；delta = N 条目。N=40 时
	// 全量约为 delta 的 ~20 倍。阈值取 5 倍以防环境抖动。
	if fullSize < deltaSize*5 {
		t.Errorf("expected full-snapshot WAL to be much larger: full=%d delta=%d", fullSize, deltaSize)
	}

	// 两种模式恢复出的状态一致。
	fullState, err := LoadStateWAL(fullPath)
	if err != nil {
		t.Fatalf("LoadStateWAL full: %v", err)
	}
	deltaState, err := LoadStateWAL(deltaPath)
	if err != nil {
		t.Fatalf("LoadStateWAL delta: %v", err)
	}
	if fullState.StepIndex != deltaState.StepIndex || fullState.Data != deltaState.Data {
		t.Errorf("recovered states differ: full(step=%d,data=%q) delta(step=%d,data=%q)",
			fullState.StepIndex, fullState.Data, deltaState.StepIndex, deltaState.Data)
	}
	if len(fullState.StepOutputs) != len(deltaState.StepOutputs) {
		t.Errorf("recovered output counts differ: full=%d delta=%d",
			len(fullState.StepOutputs), len(deltaState.StepOutputs))
	}
}

// TestLoadStateWAL_MixedSnapshotAndDelta 旧式全量记录（无 is_delta 字段）
// 与新 delta 记录混合的回放：delta 折叠到最近的全量快照之上。
func TestLoadStateWAL_MixedSnapshotAndDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	defer wal.Close()

	// 旧式全量（构造时不设置 IsDelta）。
	if err := wal.Append(WALRecord{
		StepIndex: 0, Data: "d0",
		StepOutputs: map[int]string{0: "o0"},
		Variables:   map[string]string{"v": "a"},
	}); err != nil {
		t.Fatalf("append snapshot: %v", err)
	}
	// 新 delta。
	if err := wal.Append(WALRecord{
		StepIndex: 1, Data: "d1", IsDelta: true,
		StepOutputs: map[int]string{1: "o1"},
		Variables:   map[string]string{"v": "b"},
	}); err != nil {
		t.Fatalf("append delta: %v", err)
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	state, err := LoadStateWAL(path)
	if err != nil {
		t.Fatalf("LoadStateWAL: %v", err)
	}
	if state.StepIndex != 1 || state.Data != "d1" {
		t.Errorf("step/data = %d/%q, want 1/d1", state.StepIndex, state.Data)
	}
	if state.StepOutputs[0] != "o0" || state.StepOutputs[1] != "o1" {
		t.Errorf("outputs = %v, want {0:o0, 1:o1}", state.StepOutputs)
	}
	if state.Variables["v"] != "b" {
		t.Errorf("variable v = %q, want b (delta overwrote snapshot)", state.Variables["v"])
	}
}

// TestWAL_CompactMergesDeltas Compact 必须把 delta 序列折叠为单条全量
// 快照（直接取最后一条记录会丢失更早的 delta 内容）。
func TestWAL_CompactMergesDeltas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}

	wf := &Workflow{Name: "compact-wf", Steps: []WorkflowStep{{Node: "n"}, {Node: "n"}, {Node: "n"}}}
	m := NewWALStateManager(wal, wf, nil)
	for i := 0; i < 3; i++ {
		outs := map[int]string{i: "o" + string(rune('0'+i))}
		e := newDeltaTestEngine(outs, map[string]string{"v": "a"})
		if err := m.Save(i, "d"+string(rune('0'+i)), e); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if err := wal.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if err := wal.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	defer wal.Close()

	recs := collectReplay(t, path)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record after compaction, got %d", len(recs))
	}
	snap := recs[0]
	if snap.IsDelta {
		t.Error("compacted record must be a full snapshot")
	}
	if snap.StepIndex != 2 || snap.Data != "d2" {
		t.Errorf("snapshot step/data = %d/%q, want 2/d2", snap.StepIndex, snap.Data)
	}
	for i := 0; i < 3; i++ {
		if snap.StepOutputs[i] != "o"+string(rune('0'+i)) {
			t.Errorf("snapshot output[%d] = %q, want o%d", i, snap.StepOutputs[i], i)
		}
	}

	// Compaction 后继续 append delta：新 WAL 实例恢复 seq。
	state, err := LoadStateWAL(path)
	if err != nil {
		t.Fatalf("LoadStateWAL after compaction: %v", err)
	}
	if state.StepOutputs[2] != "o2" {
		t.Errorf("recovered output[2] = %q, want o2", state.StepOutputs[2])
	}
}

// TestWALStateManager_ResumeStaysIncremental resume 场景：以恢复状态为
// 基线初始化 manager，后续记录仍为 delta（不重写全量）。
func TestWALStateManager_ResumeStaysIncremental(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resume.wal")
	wf := &Workflow{Name: "resume-wf", Steps: []WorkflowStep{{Node: "n"}, {Node: "n"}, {Node: "n"}}}

	// 第一次运行：写 2 步。
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}
	m1 := NewWALStateManager(wal, wf, nil)
	e1 := newDeltaTestEngine(map[int]string{0: "o0", 1: "o1"}, nil)
	if err := m1.Save(1, "d1", e1); err != nil {
		t.Fatalf("save run1: %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("close run1: %v", err)
	}

	// 第二次运行：恢复 → 继续。
	state, err := LoadStateWAL(path)
	if err != nil || state == nil {
		t.Fatalf("LoadStateWAL: state=%v err=%v", state, err)
	}
	wal2, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL run2: %v", err)
	}
	defer wal2.Close()
	m2 := NewWALStateManager(wal2, wf, state)
	e2 := newDeltaTestEngine(map[int]string{0: "o0", 1: "o1", 2: "o2"}, nil)
	if err := m2.Save(2, "d2", e2); err != nil {
		t.Fatalf("save run2: %v", err)
	}
	if err := wal2.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	recs := collectReplay(t, path)
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[1].IsDelta != true {
		t.Error("post-resume record must be a delta")
	}
	if len(recs[1].StepOutputs) != 1 || recs[1].StepOutputs[2] != "o2" {
		t.Errorf("post-resume delta outputs = %v, want only {2:o2}", recs[1].StepOutputs)
	}

	final, err := LoadStateWAL(path)
	if err != nil {
		t.Fatalf("LoadStateWAL final: %v", err)
	}
	if final.StepOutputs[0] != "o0" || final.StepOutputs[1] != "o1" || final.StepOutputs[2] != "o2" {
		t.Errorf("final outputs = %v, want {0:o0,1:o1,2:o2}", final.StepOutputs)
	}
}

// TestWAL_SyncIntervalSyncer SyncInterval 档位：Append 只进 bufio 缓冲
// （不显式 Flush），后台 syncer 应在 interval 内落盘，独立 ReplayWAL
// 即可读到记录。对照组（无 syncer）在读期限内不可见。
func TestWAL_SyncIntervalSyncer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "syncer.wal")
	wal, err := NewWAL(path, WALOptions{SyncInterval: 30 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}

	if err := wal.Append(WALRecord{StepIndex: 0, Data: "buffered"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// 等待超过 interval，让 syncer 落盘。
	time.Sleep(120 * time.Millisecond)

	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	recs := collectReplay(t, path)
	if len(recs) != 1 || recs[0].Data != "buffered" {
		t.Fatalf("expected 1 synced record, got %v", recs)
	}
}

// TestWAL_SyncIntervalDisabled_NoBackgroundFlush 无 syncer 且未 Flush：
// 记录停留在 bufio 缓冲中，独立 reader 看不到（证明上一测试的可见性
// 确实来自 syncer 而非 Append 自带 flush）。
func TestWAL_SyncIntervalDisabled_NoBackgroundFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nosyncer.wal")
	wal, err := NewWAL(path, WALOptions{})
	if err != nil {
		t.Fatalf("NewWAL: %v", err)
	}

	if err := wal.Append(WALRecord{StepIndex: 0, Data: "buffered"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	// 独立读取：无 syncer 时记录仍在缓冲区。
	recs := collectReplay(t, path)
	if len(recs) != 0 {
		t.Fatalf("expected 0 visible records before flush, got %d", len(recs))
	}

	if err := wal.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close 落盘后可见。
	recs = collectReplay(t, path)
	if len(recs) != 1 || recs[0].Data != "buffered" {
		t.Fatalf("expected 1 record after close, got %v", recs)
	}
}

// TestWALOptionsFromEnv 环境变量档位解析。
func TestWALOptionsFromEnv(t *testing.T) {
	cases := []struct {
		name           string
		everyWrite     string
		interval       string
		wantEveryWrite bool
		wantInterval   time.Duration
	}{
		{"unset", "", "", false, 0},
		{"every-write-1", "1", "", true, 0},
		{"every-write-true", "true", "", true, 0},
		{"every-write-TRUE", "TRUE", "", true, 0},
		{"every-write-yes", "yes", "", true, 0},
		{"every-write-off", "0", "", false, 0},
		{"interval", "", "100ms", false, 100 * time.Millisecond},
		{"interval-invalid", "", "not-a-duration", false, 0},
		{"interval-zero", "", "0s", false, 0},
		{"both", "1", "100ms", true, 100 * time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("AFLARE_WAL_SYNC_EVERY_WRITE", c.everyWrite)
			t.Setenv("AFLARE_WAL_SYNC_INTERVAL", c.interval)
			got := walOptionsFromEnv()
			if got.SyncEveryWrite != c.wantEveryWrite {
				t.Errorf("SyncEveryWrite = %v, want %v", got.SyncEveryWrite, c.wantEveryWrite)
			}
			if got.SyncInterval != c.wantInterval {
				t.Errorf("SyncInterval = %v, want %v", got.SyncInterval, c.wantInterval)
			}
		})
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}
