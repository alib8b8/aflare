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

package tenant

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTenantManager(t *testing.T) {
	tm := NewTenantManager("/tmp/test-tenants")
	if tm == nil {
		t.Fatal("NewTenantManager returned nil")
	}
	if tm.storageDir != "/tmp/test-tenants" {
		t.Errorf("expected storageDir /tmp/test-tenants, got %q", tm.storageDir)
	}
	if len(tm.tenants) != 0 {
		t.Errorf("expected empty tenants map, got %d", len(tm.tenants))
	}
}

func TestCreateTenant(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{
		MaxWorkflows:    10,
		MaxConcurrency:  5,
		MaxStorageBytes: 1024 * 1024,
	}

	// Success case.
	tenant, err := tm.CreateTenant("tenant_1", "Test Tenant", config)
	if err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}
	if tenant.ID != "tenant_1" {
		t.Errorf("expected ID tenant_1, got %q", tenant.ID)
	}
	if tenant.Name != "Test Tenant" {
		t.Errorf("expected name Test Tenant, got %q", tenant.Name)
	}
	if tenant.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Verify directories were created.
	for _, rt := range []ResourceType{ResourceTypeWorkflows, ResourceTypeHistory} {
		dir := tm.GetTenantDir("tenant_1", rt)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		}
	}

	// Duplicate ID.
	_, err = tm.CreateTenant("tenant_1", "Another", config)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got %v", err)
	}

	// Empty ID (system tenant).
	_, err = tm.CreateTenant("", "System", config)
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("expected empty ID error, got %v", err)
	}

	// Empty name.
	_, err = tm.CreateTenant("tenant_2", "", config)
	if err == nil || !strings.Contains(err.Error(), "name cannot be empty") {
		t.Errorf("expected empty name error, got %v", err)
	}

	// Invalid ID.
	_, err = tm.CreateTenant("tenant-2", "Bad", config)
	if err == nil || !strings.Contains(err.Error(), "invalid tenant ID") {
		t.Errorf("expected invalid ID error, got %v", err)
	}
}

func TestDeleteTenant(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{MaxWorkflows: 5}
	if _, err := tm.CreateTenant("del_me", "Delete Me", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Success.
	if err := tm.DeleteTenant("del_me"); err != nil {
		t.Fatalf("DeleteTenant failed: %v", err)
	}

	_, err := tm.GetTenant("del_me")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected tenant to be deleted, got %v", err)
	}

	// Delete system tenant.
	err = tm.DeleteTenant("")
	if err == nil || !strings.Contains(err.Error(), "cannot delete system tenant") {
		t.Errorf("expected system tenant delete error, got %v", err)
	}

	// Delete non-existent.
	err = tm.DeleteTenant("no_such")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got %v", err)
	}
}

func TestGetTenant(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{MaxWorkflows: 5}
	if _, err := tm.CreateTenant("get_me", "Get Me", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Success.
	tenant, err := tm.GetTenant("get_me")
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}
	if tenant.ID != "get_me" {
		t.Errorf("expected ID get_me, got %q", tenant.ID)
	}

	// Non-existent.
	_, err = tm.GetTenant("no_such")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got %v", err)
	}

	// Invalid ID.
	_, err = tm.GetTenant("bad-id")
	if err == nil || !strings.Contains(err.Error(), "invalid tenant ID") {
		t.Errorf("expected invalid ID error, got %v", err)
	}
}

func TestListTenants(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	if len(tm.ListTenants()) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(tm.ListTenants()))
	}

	config := TenantConfig{MaxWorkflows: 5}
	tm.CreateTenant("a", "A", config)
	tm.CreateTenant("b", "B", config)

	list := tm.ListTenants()
	if len(list) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(list))
	}

	// Ensure returned copies are independent.
	list[0].Name = "Modified"
	tenant, _ := tm.GetTenant(list[0].ID)
	if tenant.Name == "Modified" {
		t.Error("ListTenants should return copies")
	}
}

func TestGetTenantDir(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	// Regular tenant.
	wfDir := tm.GetTenantDir("t1", ResourceTypeWorkflows)
	expected := filepath.Join(tmpDir, "workflows", "t1")
	if wfDir != expected {
		t.Errorf("expected %q, got %q", expected, wfDir)
	}

	histDir := tm.GetTenantDir("t1", ResourceTypeHistory)
	expected = filepath.Join(tmpDir, "history", "t1")
	if histDir != expected {
		t.Errorf("expected %q, got %q", expected, histDir)
	}

	secretsPath := tm.GetTenantDir("t1", ResourceTypeSecrets)
	expected = filepath.Join(tmpDir, "secrets", "t1.enc")
	if secretsPath != expected {
		t.Errorf("expected %q, got %q", expected, secretsPath)
	}

	// System tenant.
	sysWf := tm.GetTenantDir("", ResourceTypeWorkflows)
	expected = filepath.Join(tmpDir, "workflows")
	if sysWf != expected {
		t.Errorf("expected %q, got %q", expected, sysWf)
	}

	sysSecrets := tm.GetTenantDir("", ResourceTypeSecrets)
	expected = filepath.Join(tmpDir, "secrets", "system.enc")
	if sysSecrets != expected {
		t.Errorf("expected %q, got %q", expected, sysSecrets)
	}
}

func TestValidateAccess(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{MaxWorkflows: 5}
	if _, err := tm.CreateTenant("t1", "T1", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}
	if _, err := tm.CreateTenant("t2", "T2", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Valid access within tenant directory.
	wfDir := tm.GetTenantDir("t1", ResourceTypeWorkflows)
	if err := tm.ValidateAccess("t1", wfDir); err != nil {
		t.Errorf("ValidateAccess failed for own directory: %v", err)
	}

	// Valid access to a file inside tenant directory.
	innerFile := filepath.Join(wfDir, "sub", "workflow.yaml")
	if err := tm.ValidateAccess("t1", innerFile); err != nil {
		t.Errorf("ValidateAccess failed for inner file: %v", err)
	}

	// Valid access to secrets file.
	secretsPath := tm.GetTenantDir("t1", ResourceTypeSecrets)
	if err := tm.ValidateAccess("t1", secretsPath); err != nil {
		t.Errorf("ValidateAccess failed for secrets file: %v", err)
	}

	// Cross-tenant access.
	t2Dir := tm.GetTenantDir("t2", ResourceTypeWorkflows)
	if err := tm.ValidateAccess("t1", t2Dir); err == nil {
		t.Error("expected cross-tenant access to be denied")
	}

	// Path traversal attempt.
	traversal := filepath.Join(wfDir, "..", "..", "secrets", "t2.enc")
	absTraversal, _ := filepath.Abs(traversal)
	if err := tm.ValidateAccess("t1", absTraversal); err == nil {
		t.Error("expected path traversal to be denied")
	}

	// System tenant unrestricted.
	if err := tm.ValidateAccess("", t2Dir); err != nil {
		t.Errorf("system tenant should have unrestricted access: %v", err)
	}

	// Non-existent tenant.
	if err := tm.ValidateAccess("no_such", wfDir); err == nil {
		t.Error("expected non-existent tenant to fail")
	}

	// Empty resource path.
	if err := tm.ValidateAccess("t1", ""); err == nil {
		t.Error("expected empty path to fail")
	}
}

func TestCheckQuota(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{
		MaxWorkflows:    5,
		MaxConcurrency:  3,
		MaxStorageBytes: 1000,
	}
	if _, err := tm.CreateTenant("q1", "Q1", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	// Within quota.
	usage := QuotaUsage{WorkflowCount: 3, RunningCount: 2, StorageBytes: 500}
	if err := tm.CheckQuota("q1", usage); err != nil {
		t.Errorf("CheckQuota failed within limits: %v", err)
	}

	// Exceed workflow quota.
	usage.WorkflowCount = 6
	if err := tm.CheckQuota("q1", usage); err == nil || !strings.Contains(err.Error(), "workflow quota exceeded") {
		t.Errorf("expected workflow quota error, got %v", err)
	}
	usage.WorkflowCount = 3

	// Exceed concurrency quota.
	usage.RunningCount = 4
	if err := tm.CheckQuota("q1", usage); err == nil || !strings.Contains(err.Error(), "concurrency quota exceeded") {
		t.Errorf("expected concurrency quota error, got %v", err)
	}
	usage.RunningCount = 2

	// Exceed storage quota.
	usage.StorageBytes = 1001
	if err := tm.CheckQuota("q1", usage); err == nil || !strings.Contains(err.Error(), "storage quota exceeded") {
		t.Errorf("expected storage quota error, got %v", err)
	}

	// System tenant unrestricted.
	usage = QuotaUsage{WorkflowCount: 9999, RunningCount: 9999, StorageBytes: 999999}
	if err := tm.CheckQuota("", usage); err != nil {
		t.Errorf("system tenant should have no quota limits: %v", err)
	}

	// Non-existent tenant.
	if err := tm.CheckQuota("no_such", QuotaUsage{}); err == nil {
		t.Error("expected non-existent tenant to fail")
	}
}

func TestGetTenantConfig(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	config := TenantConfig{MaxWorkflows: 42}
	if _, err := tm.CreateTenant("cfg", "Cfg", config); err != nil {
		t.Fatalf("CreateTenant failed: %v", err)
	}

	got, err := tm.GetTenantConfig("cfg")
	if err != nil {
		t.Fatalf("GetTenantConfig failed: %v", err)
	}
	if got.MaxWorkflows != 42 {
		t.Errorf("expected MaxWorkflows 42, got %d", got.MaxWorkflows)
	}

	_, err = tm.GetTenantConfig("no_such")
	if err == nil {
		t.Error("expected error for non-existent tenant")
	}
}

func TestGetTenantCacheKeyPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	if p := tm.GetTenantCacheKeyPrefix("t1"); p != "t1:" {
		t.Errorf("expected 't1:', got %q", p)
	}
	if p := tm.GetTenantCacheKeyPrefix(""); p != "" {
		t.Errorf("expected empty prefix, got %q", p)
	}
}

func TestWithTenantAndGetTenantID(t *testing.T) {
	ctx := context.Background()

	// No tenant set.
	if id := GetTenantID(ctx); id != "" {
		t.Errorf("expected empty tenant ID, got %q", id)
	}

	// Set tenant.
	ctx = WithTenant(ctx, "tenant_1")
	if id := GetTenantID(ctx); id != "tenant_1" {
		t.Errorf("expected tenant_1, got %q", id)
	}

	// Empty context.
	if id := GetTenantID(context.TODO()); id != "" {
		t.Errorf("expected empty for empty context, got %q", id)
	}
}

func TestIsValidTenantID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"", true},
		{"abc", true},
		{"ABC", true},
		{"tenant_1", true},
		{"T123", true},
		{"a_b_c", true},
		{"tenant-1", false},
		{"tenant.1", false},
		{"tenant 1", false},
		{"tenant/1", false},
		{"../etc", false},
		{strings.Repeat("a", 65), false},
	}

	for _, tt := range tests {
		if got := isValidTenantID(tt.id); got != tt.valid {
			t.Errorf("isValidTenantID(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
}

func TestTenantConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTenantManager(tmpDir)

	// Concurrent creates.
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			id := fmt.Sprintf("concurrent_%d", i)
			_, err := tm.CreateTenant(id, id, TenantConfig{})
			if err != nil {
				t.Errorf("concurrent CreateTenant failed: %v", err)
			}
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(tm.ListTenants()) != 10 {
		t.Errorf("expected 10 tenants, got %d", len(tm.ListTenants()))
	}
}
