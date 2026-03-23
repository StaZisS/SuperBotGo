package hostapi

import "context"

type HostFunction struct {
	Name        string
	Description string
	Permissions []string
	Handler     HostFunctionHandler
}

type HostFunctionHandler func(ctx context.Context, pluginID string, request []byte) (response []byte, err error)
