package hostapi

import (
	"fmt"
	"sync"
)

// ErrPermissionDenied is returned when a plugin lacks the required permission.
var ErrPermissionDenied = fmt.Errorf("permission denied")

// permissionStore tracks per-plugin granted permissions.
type permissionStore struct {
	mu    sync.RWMutex
	perms map[string]map[string]bool // pluginID -> set of permissions
}

func newPermissionStore() *permissionStore {
	return &permissionStore{
		perms: make(map[string]map[string]bool),
	}
}

// Grant registers permissions for a plugin.
func (ps *permissionStore) Grant(pluginID string, permissions []string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	set := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		set[p] = true
	}
	ps.perms[pluginID] = set
}

// Revoke removes all permissions for a plugin.
func (ps *permissionStore) Revoke(pluginID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.perms, pluginID)
}

// CheckPermission returns an error if the plugin does not have the given permission.
func (ps *permissionStore) CheckPermission(pluginID, requiredPermission string) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	perms, ok := ps.perms[pluginID]
	if !ok {
		return fmt.Errorf("%w: plugin %q has no registered permissions", ErrPermissionDenied, pluginID)
	}
	if !perms[requiredPermission] {
		return fmt.Errorf("%w: plugin %q lacks permission %q", ErrPermissionDenied, pluginID, requiredPermission)
	}
	return nil
}
