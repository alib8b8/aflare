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

package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/memory"
)

// ── MemoryCapability Tests ────────────────────────────────────────────────

func TestMemoryCapability_Init(t *testing.T) {
	m := NewMemoryCapability()
	loop := &AgentLoop{}

	err := m.Init(loop)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if m.entries == nil {
		t.Fatal("entries should be initialized")
	}
}

func TestMemoryCapability_PreProcess_Empty(t *testing.T) {
	m := NewMemoryCapability()
	_ = m.Init(&AgentLoop{})

	result, err := m.PreProcess(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result != "" {
		t.Fatal("expected empty result for empty memory")
	}
}

func TestMemoryCapability_PreProcess_WithEntries(t *testing.T) {
	m := NewMemoryCapability()
	_ = m.Init(&AgentLoop{})

	// Add some entries
	m.entries["pref_1"] = &memory.PersistentMemoryEntry{
		Key:      "pref_1",
		Value:    "I prefer Python over JavaScript",
		Category: "preference",
	}
	m.entries["fact_1"] = &memory.PersistentMemoryEntry{
		Key:      "fact_1",
		Value:    "My name is Alice",
		Category: "fact",
	}

	result, err := m.PreProcess(context.Background(), "What Python language should I use?")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result with memory entries")
	}
	if !strings.Contains(result, "Python") {
		t.Error("expected memory content in result")
	}
}

func TestMemoryCapability_PostProcess_ExtractsPreference(t *testing.T) {
	m := NewMemoryCapability()
	_ = m.Init(&AgentLoop{})

	_, _ = m.PostProcess(context.Background(), "I prefer using Go for backend", "Great choice!")
	if len(m.entries) == 0 {
		t.Fatal("expected preference to be extracted")
	}
}

func TestMemoryCapability_SearchRelevant(t *testing.T) {
	m := NewMemoryCapability()
	m.entries["pref_1"] = &memory.PersistentMemoryEntry{
		Key:      "pref_1",
		Value:    "I prefer Python",
		Category: "preference",
	}
	m.entries["fact_1"] = &memory.PersistentMemoryEntry{
		Key:      "fact_1",
		Value:    "My favorite color is blue",
		Category: "fact",
	}

	results := m.searchRelevant("What Python programming language should I use?", 5)
	if len(results) == 0 {
		t.Fatal("expected relevant results")
	}
}

func TestMemoryCapability_HasSimilarEntry(t *testing.T) {
	m := NewMemoryCapability()
	m.entries["pref_1"] = &memory.PersistentMemoryEntry{
		Key:      "pref_1",
		Value:    "I prefer Python",
		Category: "preference",
	}

	if !m.hasSimilarEntry("preference", "I prefer Python") {
		t.Error("should detect similar entry")
	}
	if m.hasSimilarEntry("preference", "completely different") {
		t.Error("should not detect unrelated entry")
	}
}

// ── PlanningCapability Tests ──────────────────────────────────────────────

func TestPlanningCapability_Init(t *testing.T) {
	p := NewPlanningCapability()
	// Use a temp path to avoid loading from real config
	p.storePath = filepath.Join(t.TempDir(), "test_plans.json")
	err := p.Init(&AgentLoop{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if p.activePlan != nil {
		t.Fatal("no active plan should exist on init")
	}
}

func TestPlanningCapability_PreProcess_NoActivePlan(t *testing.T) {
	p := NewPlanningCapability()
	p.storePath = filepath.Join(t.TempDir(), "test_plans.json")
	_ = p.Init(&AgentLoop{})

	// Short input should not trigger planning
	result, err := p.PreProcess(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result != "" {
		t.Fatal("short input should not trigger planning")
	}

	// Complex input should trigger planning
	result, err = p.PreProcess(context.Background(), "I need to create a full CI/CD pipeline with automated testing and deployment")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result == "" || !strings.Contains(result, "Planning Mode") {
		t.Fatal("complex input should trigger planning")
	}
}

func TestPlanningCapability_ExtractPlan(t *testing.T) {
	p := NewPlanningCapability()

	output := `Plan: Set up CI/CD pipeline
1. Search for existing CI templates
2. Create a workflow for testing
3. Configure deployment steps
4. Test the pipeline`

	plan := p.extractPlan(output)
	if plan == nil {
		t.Fatal("expected plan to be extracted")
	}
	if len(plan.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Status != "in_progress" {
		t.Error("first step should be in_progress")
	}
}

func TestPlanningCapability_PostProcess(t *testing.T) {
	p := NewPlanningCapability()
	_ = p.Init(&AgentLoop{})

	// Plan creation
	output := `Plan: Test task
1. Do something
2. Do another thing`

	_, err := p.PostProcess(context.Background(), "test", output)
	if err != nil {
		t.Fatalf("PostProcess failed: %v", err)
	}
	if p.activePlan == nil {
		t.Fatal("expected active plan to be created")
	}
	if len(p.activePlan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(p.activePlan.Steps))
	}
}

func TestPlanningCapability_Persist(t *testing.T) {
	// Use a temp path for testing
	p := NewPlanningCapability()
	tmpDir := t.TempDir()
	p.storePath = filepath.Join(tmpDir, "test_plans.json")

	p.activePlan = &Plan{
		ID:   "plan_test",
		Goal: "test goal",
		Steps: []PlanStep{
			{ID: 1, Goal: "step 1", Status: "done"},
			{ID: 2, Goal: "step 2", Status: "pending"},
		},
	}

	err := p.persist()
	if err != nil {
		t.Fatalf("persist failed: %v", err)
	}

	// Verify file exists and is valid
	data, err := os.ReadFile(p.storePath)
	if err != nil {
		t.Fatalf("read persisted file failed: %v", err)
	}

	var saved struct {
		Active    *Plan  `json:"active"`
		Completed []Plan `json:"completed"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("unmarshal persisted data failed: %v", err)
	}
	if saved.Active == nil || saved.Active.Goal != "test goal" {
		t.Fatal("saved plan does not match")
	}
}

// ── MultiAgentCapability Tests ────────────────────────────────────────────

func TestMultiAgentCapability_Init(t *testing.T) {
	m := NewMultiAgentCapability()
	err := m.Init(&AgentLoop{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if len(m.roles) == 0 {
		t.Fatal("roles should be initialized")
	}
}

func TestMultiAgentCapability_PreProcess_ShortInput(t *testing.T) {
	m := NewMultiAgentCapability()
	_ = m.Init(&AgentLoop{})

	result, err := m.PreProcess(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result != "" {
		t.Fatal("short input should not trigger multi-agent mode")
	}
}

func TestMultiAgentCapability_PreProcess_ComplexInput(t *testing.T) {
	m := NewMultiAgentCapability()
	_ = m.Init(&AgentLoop{})

	longInput := strings.Repeat("This is a complex task that requires multiple steps and coordination between different roles. ", 5)
	result, err := m.PreProcess(context.Background(), longInput)
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result == "" || !strings.Contains(result, "Multi-Agent Mode") {
		t.Fatal("complex input should trigger multi-agent mode")
	}
	if !m.active {
		t.Fatal("multi-agent should be active")
	}
}

func TestMultiAgentCapability_InferRole(t *testing.T) {
	m := NewMultiAgentCapability()

	tests := []struct {
		task string
		role string
	}{
		{"search for templates", "researcher"},
		{"find the answer", "researcher"},
		{"run the workflow", "executor"},
		{"create a new pipeline", "executor"},
		{"analyze the results", "analyst"},
		{"validate the output", "analyst"},
		{"combine all findings", "coordinator"},
		{"summarize the report", "coordinator"},
		{"unknown task", "executor"},
	}

	for _, tt := range tests {
		role := m.inferRole(tt.task)
		if role != tt.role {
			t.Errorf("inferRole(%q) = %q, want %q", tt.task, role, tt.role)
		}
	}
}

func TestMultiAgentCapability_ParseSubTasks(t *testing.T) {
	m := NewMultiAgentCapability()

	output := `Task decomposition:
1. Search for CI templates
2. Run the CI testing workflow
3. Analyze test results
4. Combine into a report`

	tasks := m.parseSubTasks(output)
	if len(tasks) != 4 {
		t.Fatalf("expected 4 sub-tasks, got %d", len(tasks))
	}
	if tasks[0].Role != "researcher" {
		t.Errorf("task 1 should be researcher, got %s", tasks[0].Role)
	}
	if tasks[1].Role != "executor" {
		t.Errorf("task 2 should be executor, got %s", tasks[1].Role)
	}
}

// ── WorkflowCapability Tests ──────────────────────────────────────────────

func TestWorkflowCapability_Init(t *testing.T) {
	w := NewWorkflowCapability()
	err := w.Init(&AgentLoop{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestWorkflowCapability_PreProcess(t *testing.T) {
	w := NewWorkflowCapability()
	_ = w.Init(&AgentLoop{})

	result, err := w.PreProcess(context.Background(), "create a new deployment pipeline")
	if err != nil {
		t.Fatalf("PreProcess failed: %v", err)
	}
	if result == "" || !strings.Contains(result, "Workflow Mode") {
		t.Fatal("should inject workflow guidance")
	}
	if !strings.Contains(result, "template_list") {
		t.Error("should mention template_list tool")
	}
}

func TestWorkflowCapability_ExtractTemplateName(t *testing.T) {
	w := NewWorkflowCapability()
	w.templates["ci-pipeline"] = TemplateMeta{Name: "ci-pipeline"}

	tests := []struct {
		output   string
		expected string
	}{
		{"running CI pipeline template", "ci-pipeline"},
		{"executing template: ci-pipeline", "ci-pipeline"},
		{"workflow: deploy-app completed", "deploy-app"},
	}

	for _, tt := range tests {
		name := w.extractTemplateName(tt.output)
		if strings.ToLower(name) != strings.ToLower(tt.expected) {
			t.Errorf("extractTemplateName(%q) = %q, want %q", tt.output, name, tt.expected)
		}
	}
}

func TestWorkflowCapability_SuggestsTask(t *testing.T) {
	taskInputs := []string{
		"create a new workflow",
		"build a deployment pipeline",
		"analyze the codebase",
		"configure the server",
	}
	for _, input := range taskInputs {
		if !suggestsTask(input) {
			t.Errorf("should suggest task for: %s", input)
		}
	}

	nonTaskInputs := []string{
		"hello",
		"what is the weather",
		"thanks",
	}
	for _, input := range nonTaskInputs {
		if suggestsTask(input) {
			t.Errorf("should not suggest task for: %s", input)
		}
	}
}

// ── SimulationCapability Tests ────────────────────────────────────────────

func TestSimulationCapability_Init(t *testing.T) {
	s := NewSimulationCapability()
	err := s.Init(&AgentLoop{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestSimulationCapability_IsSimulationRequest(t *testing.T) {
	s := NewSimulationCapability()

	simInputs := []string{
		"simulate a conversation between a doctor and patient",
		"role-play as a teacher explaining math",
		"pretend you are a medieval knight",
	}
	for _, input := range simInputs {
		if !s.isSimulationRequest(input) {
			t.Errorf("should detect simulation request: %s", input)
		}
	}

	nonSimInputs := []string{
		"hello",
		"what is the weather",
		"you are a helpful assistant",
		"create a workflow",
	}
	for _, input := range nonSimInputs {
		if s.isSimulationRequest(input) {
			t.Errorf("should not detect simulation request: %s", input)
		}
	}
}

func TestSimulationCapability_InitializeSimulation(t *testing.T) {
	s := NewSimulationCapability()

	input := "simulate a wise old wizard named Merlin, role: advisor, traits: wise, patient, mysterious, background: lives in a tower"
	state := s.initializeSimulation(input)

	if state.Persona.Name != "Merlin" {
		t.Errorf("expected name Merlin, got %s", state.Persona.Name)
	}
	if state.Persona.Role != "advisor" {
		t.Errorf("expected role advisor, got %s", state.Persona.Role)
	}
	if len(state.Persona.Traits) == 0 {
		t.Error("should have traits")
	}
	if state.Turn != 0 {
		t.Errorf("expected turn 0, got %d", state.Turn)
	}
}

func TestSimulationCapability_ExtractEvents(t *testing.T) {
	s := NewSimulationCapability()

	output := `* The wizard strokes his beard thoughtfully
- He opens the ancient tome
* A spell is cast with a flash of light`

	events := s.extractEvents(output)
	if len(events) == 0 {
		t.Fatal("should extract events")
	}
}

func TestSimulationCapability_IsSimulationEnd(t *testing.T) {
	s := NewSimulationCapability()

	if !s.isSimulationEnd("End simulation") {
		t.Error("should detect end simulation")
	}
	if !s.isSimulationEnd("The simulation is complete.") {
		t.Error("should detect simulation complete")
	}
	if s.isSimulationEnd("Let's continue the conversation") {
		t.Error("should not detect end in normal text")
	}
}

// ── Learning System Tests ─────────────────────────────────────────────────

func TestLearningStore_Dedup(t *testing.T) {
	store := &learningStore{
		maxRecentKeys: 10,
		appendCount:   0,
	}

	// Add first entry
	entry1 := LearningEntry{
		Capability: "reflection",
		Input:      "build a CI pipeline",
		Issues:     []string{"output is too short"},
	}
	store.append(entry1)

	// Similar entry should be skipped
	entry2 := LearningEntry{
		Capability: "reflection",
		Input:      "build a CI pipeline",
		Issues:     []string{"output is too short"},
	}
	store.append(entry2)

	// The second entry should have been deduped
	if len(store.recentKeys) != 1 {
		t.Errorf("expected 1 recent key after dedup, got %d", len(store.recentKeys))
	}
}

func TestLearningStore_BuildEntryKey(t *testing.T) {
	store := &learningStore{}

	entry := LearningEntry{
		Input:  "build a CI pipeline",
		Issues: []string{"output too short", "missing action"},
	}

	key := store.buildEntryKey(entry)
	if !strings.Contains(key, "build a ci pipeline") {
		t.Error("key should contain input")
	}
	if !strings.Contains(key, "output too short") {
		t.Error("key should contain issues")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		sim  float64
		min  float64
	}{
		{"hello world", "hello world", 1.0, 0.99},
		{"hello world", "hello", 0.5, 0.4},
		{"hello world", "goodbye mars", 0.0, 0.0},
		{"build CI pipeline", "build CI pipeline for testing", 0.6, 0.5},
	}

	for _, tt := range tests {
		sim := jaccardSimilarity(toWordSet(tt.a), toWordSet(tt.b))
		if sim < tt.min {
			t.Errorf("jaccard(%q, %q) = %f, want >= %f", tt.a, tt.b, sim, tt.min)
		}
	}
}

func TestNormalizeIssue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"output is too short or empty", "output is too short or empty"},
		{"response lacks concrete actions or tool calls", "response lacks concrete actions or tool"},
		{"agent is refusing or unable to help", "agent is refusing or unable to"},
	}

	for _, tt := range tests {
		result := normalizeIssue(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeIssue(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ── Capability Registry Tests ─────────────────────────────────────────────

func TestCapabilityRegistry_RegisterAndGet(t *testing.T) {
	cr := NewCapabilityRegistry()
	mem := NewMemoryCapability()

	cr.Register(mem)

	if cr.Count() != 1 {
		t.Errorf("expected 1 capability, got %d", cr.Count())
	}

	if cr.Get("memory") == nil {
		t.Error("should get memory capability")
	}

	if cr.Get("nonexistent") != nil {
		t.Error("should return nil for nonexistent capability")
	}
}

func TestCapabilityRegistry_Names(t *testing.T) {
	cr := NewCapabilityRegistry()
	cr.Register(NewMemoryCapability())
	cr.Register(NewPlanningCapability())

	names := cr.Names()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestCapabilityRegistry_PreProcessAll(t *testing.T) {
	cr := NewCapabilityRegistry()
	cr.Register(NewMemoryCapability())
	_ = cr.InitAll(&AgentLoop{})

	result, err := cr.PreProcessAll(context.Background(), "hello")
	if err != nil {
		t.Fatalf("PreProcessAll failed: %v", err)
	}
	// Memory is empty, so result should be unchanged
	if result != "hello" {
		t.Errorf("expected unchanged input, got %q", result)
	}
}

func TestParseCapabilities(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"all", 10},
		{"", 0},
		{"memory,planning", 2},
		{"memory,invalid,planning", 2},
	}

	for _, tt := range tests {
		result := ParseCapabilities(tt.input)
		if len(result) != tt.expected {
			t.Errorf("ParseCapabilities(%q) = %d, want %d", tt.input, len(result), tt.expected)
		}
	}
}

func TestCreateCapability(t *testing.T) {
	tests := []string{
		"reflection", "human-in-loop", "bdi", "utility",
		"adaptive", "memory", "planning", "multi-agent",
		"workflow", "simulation",
	}

	for _, name := range tests {
		cap := CreateCapability(name)
		if cap == nil {
			t.Errorf("CreateCapability(%q) should not return nil", name)
		}
		if cap.Name() != name {
			t.Errorf("CreateCapability(%q).Name() = %q", name, cap.Name())
		}
	}

	if CreateCapability("nonexistent") != nil {
		t.Error("CreateCapability for nonexistent should return nil")
	}
}

// ── Helper Function Tests ─────────────────────────────────────────────────

func TestIsStepLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"1. do something", true},
		{"1) do something", true},
		{"Step 1: do something", true},
		{"hello world", false},
		{"plain text", false},
	}

	for _, tt := range tests {
		if isStepLine(tt.line) != tt.expected {
			t.Errorf("isStepLine(%q) = %v, want %v", tt.line, !tt.expected, tt.expected)
		}
	}
}

func TestExtractStepGoal(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"1. search for templates", "search for templates"},
		{"1) run the workflow", "run the workflow"},
		{"Step 1: create a pipeline", "create a pipeline"},
	}

	for _, tt := range tests {
		goal := extractStepGoal(tt.line)
		if goal != tt.expected {
			t.Errorf("extractStepGoal(%q) = %q, want %q", tt.line, goal, tt.expected)
		}
	}
}

func TestContainsActionVerb(t *testing.T) {
	actions := []string{"create a file", "build a pipeline", "deploy to production", "analyze the data"}
	for _, a := range actions {
		if !containsActionVerb(a) {
			t.Errorf("should detect action verb in: %s", a)
		}
	}

	nonActions := []string{"hello", "what time is it", "thank you"}
	for _, na := range nonActions {
		if containsActionVerb(na) {
			t.Errorf("should not detect action verb in: %s", na)
		}
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello_world"},
		{"I'm a test", "im_a_test"},
		{"my_name", "my_name"},
	}

	for _, tt := range tests {
		result := sanitizeKey(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTokenize(t *testing.T) {
	words := tokenize("Hello, World! This is a test.")
	if len(words) == 0 {
		t.Fatal("should have tokens")
	}
	// Check that punctuation is stripped
	for _, w := range words {
		if strings.ContainsAny(w, ".,!?;:()[]{}'\"") {
			t.Errorf("token %q should not contain punctuation", w)
		}
	}
}

// ── Integration: Full Capability Chain ────────────────────────────────────

func TestIntegration_FullCapabilityChain(t *testing.T) {
	cr := NewCapabilityRegistry()

	// Register all capabilities
	capabilities := []AgentCapability{
		NewMemoryCapability(),
		NewPlanningCapability(),
		NewReflectionCapability(),
		NewBDICapability(),
		NewMultiAgentCapability(),
		NewWorkflowCapability(),
		NewSimulationCapability(),
	}

	for _, cap := range capabilities {
		cr.Register(cap)
	}

	loop := &AgentLoop{config: Config{MaxIterations: 10}}
	if err := cr.InitAll(loop); err != nil {
		t.Fatalf("InitAll failed: %v", err)
	}

	if cr.Count() != 7 {
		t.Errorf("expected 7 capabilities, got %d", cr.Count())
	}

	// Test PreProcess chain
	complexInput := "I need to create a full CI/CD pipeline with automated testing and deployment to Kubernetes"
	result, err := cr.PreProcessAll(context.Background(), complexInput)
	if err != nil {
		t.Fatalf("PreProcessAll failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result from capability chain")
	}

	// Test PostProcess chain
	processed, err := cr.PostProcessAll(context.Background(), complexInput, "I found the ci-pipeline template and ran it successfully")
	if err != nil {
		t.Fatalf("PostProcessAll failed: %v", err)
	}
	_ = processed // may or may not be modified

	// Test Shutdown
	if err := cr.ShutdownAll(); err != nil {
		t.Fatalf("ShutdownAll failed: %v", err)
	}
}

func TestIntegration_MemoryWithPlanningPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up memory with plan persistence
	mem := NewMemoryCapability()
	plan := NewPlanningCapability()
	plan.storePath = filepath.Join(tmpDir, "plans.json")

	loop := &AgentLoop{config: Config{MaxIterations: 10}}

	if err := mem.Init(loop); err != nil {
		t.Fatalf("memory init failed: %v", err)
	}
	if err := plan.Init(loop); err != nil {
		t.Fatalf("planning init failed: %v", err)
	}

	// Simulate a conversation turn
	ctx := context.Background()
	input := "I prefer dark mode and I need to build a new dashboard"

	// Memory should extract the preference
	_, _ = mem.PostProcess(ctx, input, "I'll help you build a dashboard with dark mode")

	if len(mem.entries) == 0 {
		t.Fatal("memory should have extracted preference")
	}

	// Planning should be triggered
	_, _ = plan.PostProcess(ctx, input, "Plan:\n1. Search for dashboard templates\n2. Create the dashboard workflow\n3. Test the dashboard")

	if plan.activePlan == nil {
		t.Fatal("planning should have extracted plan")
	}
}