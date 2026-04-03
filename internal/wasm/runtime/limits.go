package runtime

import "time"

// Plugin resource limits — collected here so the full "contract"
// between the platform and WASM plugins is visible in one place.
const (
	// KV store.
	KVMaxKeysPerPlugin  = 1000
	KVMaxValueSize      = 64 * 1024        // 64 KB
	KVMaxTotalPerPlugin = 10 * 1024 * 1024 // 10 MB

	// File API.
	MaxFileChunkSize = 1 << 20          // 1 MB per read chunk
	MaxFileStoreSize = 50 * 1024 * 1024 // 50 MB max file size via host API

	// SQL.
	SQLMaxHandlesPerExecution = 16

	// Inter-plugin calls.
	MaxCallDepth = 5

	// Graceful shutdown.
	PluginDrainTimeout = 10 * time.Second
)
