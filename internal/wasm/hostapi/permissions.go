package hostapi

import (
	"fmt"
	"sync"
)

var ErrPermissionDenied = fmt.Errorf("permission denied")

type permissionStore struct {
	mu    sync.RWMutex
	perms map[string]map[string]bool
}

func newPermissionStore() *permissionStore {
	return &permissionStore{
		perms: make(map[string]map[string]bool),
	}
}

func (ps *permissionStore) Grant(pluginID string, permissions []string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	set := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		set[p] = true
	}
	ps.perms[pluginID] = set
}

func (ps *permissionStore) Revoke(pluginID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.perms, pluginID)
}

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
