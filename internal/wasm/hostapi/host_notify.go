package hostapi

import (
	"context"
	"time"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

var wasmNotifyMaxTimeout = time.Duration(wasmrt.DefaultHostNotifyTimeoutSeconds) * time.Second

// ---------------------------------------------------------------------------
// notify_user
// ---------------------------------------------------------------------------

type notifyUserRequest struct {
	UserID   int64  `msgpack:"user_id"`
	Text     string `msgpack:"text"`
	Priority int    `msgpack:"priority"`
}

type notifyResponse struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

func (h *HostAPI) notifyUserFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyUserRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyUser(reqCtx, req.UserID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

// ---------------------------------------------------------------------------
// notify_chat
// ---------------------------------------------------------------------------

type notifyChatRequest struct {
	ChannelType string `msgpack:"channel_type"`
	ChatID      string `msgpack:"chat_id"`
	Text        string `msgpack:"text"`
	Priority    int    `msgpack:"priority"`
}

func (h *HostAPI) notifyChatFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyChatRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyChat(reqCtx, req.ChannelType, req.ChatID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

// ---------------------------------------------------------------------------
// notify_project
// ---------------------------------------------------------------------------

type notifyProjectRequest struct {
	ProjectID int64  `msgpack:"project_id"`
	Text      string `msgpack:"text"`
	Priority  int    `msgpack:"priority"`
}

func (h *HostAPI) notifyProjectFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyProjectRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyProject(reqCtx, req.ProjectID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func clampPriority(p int) int {
	if p < 0 {
		return 0
	}
	if p > 3 {
		return 3
	}
	return p
}
