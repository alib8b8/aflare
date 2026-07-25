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
	"sync"
	"time"
)

// ResourceType represents the type of tenant-isolated resource.
type ResourceType string

const (
	ResourceTypeWorkflows ResourceType = "workflows"
	ResourceTypeHistory   ResourceType = "history"
	ResourceTypeSecrets   ResourceType = "secrets"
)

// Tenant represents a single tenant.
type Tenant struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	CreatedAt time.Time              `json:"created_at"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

// TenantConfig defines resource quotas for a tenant.
type TenantConfig struct {
	MaxWorkflows    int   `json:"max_workflows"`
	MaxConcurrency  int   `json:"max_concurrency"`
	MaxStorageBytes int64 `json:"max_storage_bytes"`
}

// tenantContextKey is the key type for tenant IDs in context.
type tenantContextKey struct{}

// QuotaUsage tracks current resource consumption for quota checks.
type QuotaUsage struct {
	WorkflowCount int
	RunningCount  int
	StorageBytes  int64
}

// TenantManager manages tenant lifecycle and resource isolation.
type TenantManager struct {
	storageDir string
	tenants    map[string]*Tenant
	configs    map[string]TenantConfig
	mu         sync.RWMutex
}

// NewTenantManager creates a new TenantManager.
func NewTenantManager(storageDir string) *TenantManager {
	return &TenantManager{
		storageDir: storageDir,
		tenants:    make(map[string]*Tenant),
		configs:    make(map[string]TenantConfig),
	}
}

// isValidTenantID validates a tenant ID (alphanumeric and underscore only).
// Empty string is valid and represents the system tenant.
func isValidTenantID(id string) bool {
	if id == "" {
		return true
	}
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// CreateTenant creates a new tenant with the given configuration.
func (tm *TenantManager) CreateTenant(id, name string, config TenantConfig) (*Tenant, error) {
	if !isValidTenantID(id) {
		return nil, fmt.Errorf("invalid tenant ID: %q", id)
	}
	if id == "" {
		return nil, fmt.Errorf("tenant ID cannot be empty (empty ID is reserved for system tenant)")
	}
	if name == "" {
		return nil, fmt.Errorf("tenant name cannot be empty")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tenants[id]; exists {
		return nil, fmt.Errorf("tenant %q already exists", id)
	}

	tenant := &Tenant{
		ID:        id,
		Name:      name,
		CreatedAt: time.Now(),
		Config:    make(map[string]interface{}),
	}

	// Create resource directories.
	for _, rt := range []ResourceType{ResourceTypeWorkflows, ResourceTypeHistory, ResourceTypeSecrets} {
		path := tm.getTenantResourcePath(id, rt)
		if rt == ResourceTypeSecrets {
			// Secrets is a file; ensure parent directory exists.
			parent := filepath.Dir(path)
			if err := os.MkdirAll(parent, 0750); err != nil {
				return nil, fmt.Errorf("failed to create secrets directory for tenant %q: %w", id, err)
			}
		} else {
			if err := os.MkdirAll(path, 0750); err != nil {
				return nil, fmt.Errorf("failed to create %s directory for tenant %q: %w", rt, id, err)
			}
		}
	}

	tm.tenants[id] = tenant
	tm.configs[id] = config

	return tenant, nil
}

// DeleteTenant removes a tenant and its resources.
func (tm *TenantManager) DeleteTenant(id string) error {
	if id == "" {
		return fmt.Errorf("cannot delete system tenant")
	}
	if !isValidTenantID(id) {
		return fmt.Errorf("invalid tenant ID: %q", id)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tenants[id]; !exists {
		return fmt.Errorf("tenant %q does not exist", id)
	}

	delete(tm.tenants, id)
	delete(tm.configs, id)

	// Remove resource directories.
	for _, rt := range []ResourceType{ResourceTypeWorkflows, ResourceTypeHistory, ResourceTypeSecrets} {
		path := tm.getTenantResourcePath(id, rt)
		_ = os.RemoveAll(path)
		// Best-effort cleanup of empty parent directory.
		if rt != ResourceTypeSecrets {
			parent := filepath.Dir(path)
			_ = os.Remove(parent)
		}
	}

	return nil
}

// GetTenant retrieves a tenant by ID.
func (tm *TenantManager) GetTenant(id string) (*Tenant, error) {
	if !isValidTenantID(id) {
		return nil, fmt.Errorf("invalid tenant ID: %q", id)
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tenant, exists := tm.tenants[id]
	if !exists {
		return nil, fmt.Errorf("tenant %q does not exist", id)
	}

	// Return a copy to prevent external mutation.
	t := *tenant
	if tenant.Config != nil {
		t.Config = make(map[string]interface{}, len(tenant.Config))
		for k, v := range tenant.Config {
			t.Config[k] = v
		}
	}
	return &t, nil
}

// ListTenants returns all registered tenants.
func (tm *TenantManager) ListTenants() []*Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	list := make([]*Tenant, 0, len(tm.tenants))
	for _, tenant := range tm.tenants {
		t := *tenant
		if tenant.Config != nil {
			t.Config = make(map[string]interface{}, len(tenant.Config))
			for k, v := range tenant.Config {
				t.Config[k] = v
			}
		}
		list = append(list, &t)
	}
	return list
}

// getTenantResourcePath returns the filesystem path for a tenant's resource.
func (tm *TenantManager) getTenantResourcePath(tenantID string, resourceType ResourceType) string {
	if tenantID == "" {
		// System tenant uses the root resource directories.
		switch resourceType {
		case ResourceTypeWorkflows:
			return filepath.Join(tm.storageDir, string(ResourceTypeWorkflows))
		case ResourceTypeHistory:
			return filepath.Join(tm.storageDir, string(ResourceTypeHistory))
		case ResourceTypeSecrets:
			return filepath.Join(tm.storageDir, string(ResourceTypeSecrets), "system.enc")
		default:
			return filepath.Join(tm.storageDir, string(resourceType))
		}
	}

	switch resourceType {
	case ResourceTypeWorkflows:
		return filepath.Join(tm.storageDir, string(ResourceTypeWorkflows), tenantID)
	case ResourceTypeHistory:
		return filepath.Join(tm.storageDir, string(ResourceTypeHistory), tenantID)
	case ResourceTypeSecrets:
		return filepath.Join(tm.storageDir, string(ResourceTypeSecrets), tenantID+".enc")
	default:
		return filepath.Join(tm.storageDir, string(resourceType), tenantID)
	}
}

// GetTenantDir returns the filesystem path for a tenant's resource directory.
func (tm *TenantManager) GetTenantDir(tenantID string, resourceType ResourceType) string {
	return tm.getTenantResourcePath(tenantID, resourceType)
}

// ValidateAccess ensures the resource path belongs to the specified tenant.
func (tm *TenantManager) ValidateAccess(tenantID string, resourcePath string) error {
	if tenantID == "" {
		return nil // System tenant has unrestricted access.
	}
	if !isValidTenantID(tenantID) {
		return fmt.Errorf("invalid tenant ID: %q", tenantID)
	}

	tm.mu.RLock()
	_, exists := tm.tenants[tenantID]
	tm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("tenant %q does not exist", tenantID)
	}

	if resourcePath == "" {
		return fmt.Errorf("empty resource path")
	}

	absResource, err := filepath.Abs(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve resource path: %w", err)
	}

	// Check against all resource types for this tenant.
	resourceTypes := []ResourceType{ResourceTypeWorkflows, ResourceTypeHistory, ResourceTypeSecrets}
	for _, rt := range resourceTypes {
		tenantPath := tm.getTenantResourcePath(tenantID, rt)
		absTenantPath, err := filepath.Abs(tenantPath)
		if err != nil {
			continue
		}

		// Exact match for files (secrets).
		if absResource == absTenantPath {
			return nil
		}

		// Directory containment check.
		rel, err := filepath.Rel(absTenantPath, absResource)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(rel, "..") && rel != ".." {
			return nil
		}
	}

	return fmt.Errorf("resource path %q does not belong to tenant %q", resourcePath, tenantID)
}

// CheckQuota validates whether the tenant's current usage is within configured quotas.
func (tm *TenantManager) CheckQuota(tenantID string, usage QuotaUsage) error {
	if tenantID == "" {
		return nil // System tenant has no quota restrictions.
	}
	if !isValidTenantID(tenantID) {
		return fmt.Errorf("invalid tenant ID: %q", tenantID)
	}

	tm.mu.RLock()
	config, exists := tm.configs[tenantID]
	tm.mu.RUnlock()
	if !exists {
		return fmt.Errorf("tenant %q does not exist", tenantID)
	}

	if config.MaxWorkflows > 0 && usage.WorkflowCount > config.MaxWorkflows {
		return fmt.Errorf("workflow quota exceeded: %d/%d", usage.WorkflowCount, config.MaxWorkflows)
	}
	if config.MaxConcurrency > 0 && usage.RunningCount > config.MaxConcurrency {
		return fmt.Errorf("concurrency quota exceeded: %d/%d", usage.RunningCount, config.MaxConcurrency)
	}
	if config.MaxStorageBytes > 0 && usage.StorageBytes > config.MaxStorageBytes {
		return fmt.Errorf("storage quota exceeded: %d/%d bytes", usage.StorageBytes, config.MaxStorageBytes)
	}

	return nil
}

// GetTenantConfig returns the configuration for a tenant.
func (tm *TenantManager) GetTenantConfig(tenantID string) (TenantConfig, error) {
	if !isValidTenantID(tenantID) {
		return TenantConfig{}, fmt.Errorf("invalid tenant ID: %q", tenantID)
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	config, exists := tm.configs[tenantID]
	if !exists {
		return TenantConfig{}, fmt.Errorf("tenant %q does not exist", tenantID)
	}
	return config, nil
}

// GetTenantCacheKeyPrefix returns the cache key prefix for a tenant.
func (tm *TenantManager) GetTenantCacheKeyPrefix(tenantID string) string {
	if tenantID == "" {
		return ""
	}
	return tenantID + ":"
}

// WithTenant wraps a context with the given tenant ID.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// GetTenantID extracts the tenant ID from a context.
// Returns empty string if no tenant ID is set (system tenant).
func GetTenantID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(tenantContextKey{}).(string); ok {
		return id
	}
	return ""
}
