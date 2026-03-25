package hostapi

import (
	"context"
	"time"

	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
)

var wasmNotifyMaxTimeout = time.Duration(wasmrt.DefaultHostNotifyTimeoutSeconds) * time.Second

// ---------------------------------------------------------------------------
// notify_user
// ---------------------------------------------------------------------------

type notifyUserRequest struct {
	UserID   int64  `json:"user_id" msgpack:"user_id"`
	Text     string `json:"text" msgpack:"text"`
	Priority int    `json:"priority" msgpack:"priority"`
}

type notifyResponse struct {
	OK    bool   `json:"ok" msgpack:"ok"`
	Error string `json:"error,omitempty" msgpack:"error,omitempty"`
}

func (h *HostAPI) notifyUserFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyUserRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.Notifier == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("Notifier"), enc)
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyUser(reqCtx, req.UserID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()}, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, notifyResponse{OK: true}, enc)
	}
}

// ---------------------------------------------------------------------------
// notify_chat
// ---------------------------------------------------------------------------

type notifyChatRequest struct {
	ChannelType string `json:"channel_type" msgpack:"channel_type"`
	ChatID      string `json:"chat_id" msgpack:"chat_id"`
	Text        string `json:"text" msgpack:"text"`
	Priority    int    `json:"priority" msgpack:"priority"`
}

func (h *HostAPI) notifyChatFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyChatRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.Notifier == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("Notifier"), enc)
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyChat(reqCtx, req.ChannelType, req.ChatID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()}, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, notifyResponse{OK: true}, enc)
	}
}

// ---------------------------------------------------------------------------
// notify_project
// ---------------------------------------------------------------------------

type notifyProjectRequest struct {
	ProjectID int64  `json:"project_id" msgpack:"project_id"`
	Text      string `json:"text" msgpack:"text"`
	Priority  int    `json:"priority" msgpack:"priority"`
}

func (h *HostAPI) notifyProjectFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, enc, err := readModMemoryAndDetect(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyProjectRequest
		if err := unmarshalPayload(data, enc, &req); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnErrorEnc(ctx, mod, stack, err, enc)
			return
		}

		if h.deps.Notifier == nil {
			returnErrorEnc(ctx, mod, stack, errDepNotAvailable("Notifier"), enc)
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyProject(reqCtx, req.ProjectID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeEncodedResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()}, enc)
			return
		}

		writeEncodedResult(ctx, mod, stack, notifyResponse{OK: true}, enc)
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
