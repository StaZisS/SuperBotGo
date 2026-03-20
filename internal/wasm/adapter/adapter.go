package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

// Compile-time check: WasmPlugin must satisfy the plugin.Plugin interface.
var _ plugin.Plugin = (*WasmPlugin)(nil)

// ReplyFunc sends a text reply back to the chat that issued the command.
type ReplyFunc func(ctx context.Context, req model.CommandRequest, text string) error

// SendFunc sends a message to an arbitrary chat on the same channel.
type SendFunc func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error

// WasmPlugin wraps a compiled Wasm module and implements the plugin.Plugin interface.
// Each call creates a new one-shot module instance (fast due to AOT compilation).
type WasmPlugin struct {
	compiled *wasmrt.CompiledModule
	meta     wasmrt.PluginMeta
	config   json.RawMessage
	reply    ReplyFunc
	send     SendFunc
}

// ID returns the plugin's unique identifier.
func (wp *WasmPlugin) ID() string {
	return wp.meta.ID
}

// Name returns the human-readable plugin name.
func (wp *WasmPlugin) Name() string {
	return wp.meta.Name
}

// Version returns the plugin version.
func (wp *WasmPlugin) Version() string {
	return wp.meta.Version
}

// SupportedRoles extracts unique roles from the plugin's command definitions.
func (wp *WasmPlugin) SupportedRoles() []string {
	seen := make(map[string]bool)
	var roles []string
	for _, cmd := range wp.meta.Commands {
		if cmd.MinRole != "" && !seen[cmd.MinRole] {
			seen[cmd.MinRole] = true
			roles = append(roles, cmd.MinRole)
		}
	}
	return roles
}

// Commands converts the plugin's command definitions to state.CommandDefinition.
func (wp *WasmPlugin) Commands() []*state.CommandDefinition {
	defs := make([]*state.CommandDefinition, len(wp.meta.Commands))
	for i, cmd := range wp.meta.Commands {
		def := &state.CommandDefinition{
			Name:        cmd.Name,
			Description: cmd.Description,
		}
		if cmd.MinRole != "" {
			def.Requirements = &model.RoleRequirements{
				GlobalRoles: []string{cmd.MinRole},
			}
		}

		if len(cmd.Nodes) > 0 {
			// New node-based command flow.
			for _, nd := range cmd.Nodes {
				if cn := wp.nodeDefToCommandNode(nd); cn != nil {
					def.Nodes = append(def.Nodes, cn)
				}
			}
		} else {
			// Legacy step-based command flow.
			for _, step := range cmd.Steps {
				def.Nodes = append(def.Nodes, stepDefToNode(step))
			}
		}

		defs[i] = def
	}
	return defs
}

// ---------------------------------------------------------------------------
// Legacy step conversion (backward compat)
// ---------------------------------------------------------------------------

// stepDefToNode converts a wasm StepDef to a state.StepNode.
func stepDefToNode(sd wasmrt.StepDef) state.StepNode {
	node := state.StepNode{
		ParamName: sd.Param,
	}

	prompt := sd.Prompt
	options := sd.Options
	node.MessageBuilder = func(_ state.StepContext) model.Message {
		if len(options) > 0 {
			opts := make([]model.Option, len(options))
			for i, o := range options {
				opts[i] = model.Option{Label: o.Label, Value: o.Value}
			}
			return model.Message{
				Blocks: []model.ContentBlock{
					model.OptionsBlock{Prompt: prompt, Options: opts},
				},
			}
		}
		return model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: prompt, Style: model.StylePlain},
			},
		}
	}

	if sd.Validation != "" {
		pattern := sd.Validation
		node.Validate = func(input model.UserInput) bool {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return true
			}
			return re.MatchString(input.TextValue())
		}
	}

	return node
}

// ---------------------------------------------------------------------------
// Node tree conversion
// ---------------------------------------------------------------------------

// nodeDefToCommandNode recursively converts a NodeDef to a state.CommandNode.
// Returns nil for unknown node types.
func (wp *WasmPlugin) nodeDefToCommandNode(nd wasmrt.NodeDef) state.CommandNode {
	switch nd.Type {
	case "step":
		return wp.stepNodeDefToStepNode(nd)
	case "branch":
		return wp.branchNodeDefToBranchNode(nd)
	case "conditional_branch":
		return wp.condBranchNodeDefToCondBranchNode(nd)
	default:
		slog.Warn("wasm: unknown node type, skipping", "plugin", wp.meta.ID, "type", nd.Type)
		return nil
	}
}

// stepNodeDefToStepNode converts a step NodeDef to a state.StepNode.
func (wp *WasmPlugin) stepNodeDefToStepNode(nd wasmrt.NodeDef) state.StepNode {
	node := state.StepNode{
		ParamName: nd.Param,
	}

	// --- MessageBuilder ---
	if len(nd.Blocks) > 0 {
		blocks := nd.Blocks
		node.MessageBuilder = func(ctx state.StepContext) model.Message {
			var contentBlocks []model.ContentBlock
			for _, b := range blocks {
				switch b.Type {
				case "text":
					contentBlocks = append(contentBlocks, model.TextBlock{
						Text:  b.Text,
						Style: parseTextStyle(b.Style),
					})
				case "options":
					opts := make([]model.Option, len(b.Options))
					for j, o := range b.Options {
						opts[j] = model.Option{Label: o.Label, Value: o.Value}
					}
					contentBlocks = append(contentBlocks, model.OptionsBlock{
						Prompt:  b.Prompt,
						Options: opts,
					})
				case "dynamic_options":
					opts := wp.callOptionsCallback(b.OptionsFn, ctx)
					contentBlocks = append(contentBlocks, model.OptionsBlock{
						Prompt:  b.Prompt,
						Options: opts,
					})
				case "link":
					contentBlocks = append(contentBlocks, model.LinkBlock{
						URL:   b.URL,
						Label: b.Label,
					})
				case "image":
					contentBlocks = append(contentBlocks, model.ImageBlock{URL: b.URL})
				}
			}
			return model.Message{Blocks: contentBlocks}
		}
	} else {
		// Fallback: empty message.
		node.MessageBuilder = func(_ state.StepContext) model.Message {
			return model.Message{}
		}
	}

	// --- Validation ---
	if nd.Validation != "" {
		pattern := nd.Validation
		node.Validate = func(input model.UserInput) bool {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return true
			}
			return re.MatchString(input.TextValue())
		}
	}
	// ValidateFn takes precedence over regex if both are present.
	if nd.ValidateFn != "" {
		cbName := nd.ValidateFn
		node.Validate = func(input model.UserInput) bool {
			return wp.callValidateCallback(cbName, input.TextValue())
		}
	}

	// --- Visibility condition ---
	if nd.VisibleWhen != nil {
		cond := nd.VisibleWhen
		node.Condition = func(params model.OptionMap) bool {
			return evalCondition(cond, params)
		}
	}
	// ConditionFn takes precedence over declarative if both are present.
	if nd.ConditionFn != "" {
		cbName := nd.ConditionFn
		node.Condition = func(params model.OptionMap) bool {
			return wp.callConditionCallback(cbName, params)
		}
	}

	// --- Pagination ---
	if nd.Pagination != nil {
		pag := nd.Pagination
		cbName := pag.Provider
		node.Pagination = &state.PaginationConfig{
			Prompt:   pag.Prompt,
			PageSize: pag.PageSize,
			PageProvider: func(ctx state.StepContext, page int) state.OptionsPage {
				return wp.callPaginationCallback(cbName, ctx, page)
			},
		}
	}

	return node
}

// branchNodeDefToBranchNode converts a branch NodeDef to a state.BranchNode.
func (wp *WasmPlugin) branchNodeDefToBranchNode(nd wasmrt.NodeDef) state.BranchNode {
	bn := state.BranchNode{
		OnParam: nd.OnParam,
		Cases:   make(map[string][]state.CommandNode),
	}
	for value, children := range nd.Cases {
		var nodes []state.CommandNode
		for _, child := range children {
			if cn := wp.nodeDefToCommandNode(child); cn != nil {
				nodes = append(nodes, cn)
			}
		}
		bn.Cases[value] = nodes
	}
	if len(nd.Default) > 0 {
		var nodes []state.CommandNode
		for _, child := range nd.Default {
			if cn := wp.nodeDefToCommandNode(child); cn != nil {
				nodes = append(nodes, cn)
			}
		}
		bn.Default = nodes
	}
	return bn
}

// condBranchNodeDefToCondBranchNode converts a conditional_branch NodeDef to
// a state.ConditionalBranchNode.
func (wp *WasmPlugin) condBranchNodeDefToCondBranchNode(nd wasmrt.NodeDef) state.ConditionalBranchNode {
	cbn := state.ConditionalBranchNode{}
	for _, cc := range nd.ConditionalCases {
		childNodes := make([]state.CommandNode, 0, len(cc.Nodes))
		for _, child := range cc.Nodes {
			if cn := wp.nodeDefToCommandNode(child); cn != nil {
				childNodes = append(childNodes, cn)
			}
		}

		var predicate func(model.OptionMap) bool
		if cc.Condition != nil {
			cond := cc.Condition
			predicate = func(params model.OptionMap) bool {
				return evalCondition(cond, params)
			}
		}
		if cc.ConditionFn != "" {
			cbName := cc.ConditionFn
			predicate = func(params model.OptionMap) bool {
				return wp.callConditionCallback(cbName, params)
			}
		}

		if predicate != nil {
			cbn.Cases = append(cbn.Cases, state.ConditionalCase{
				Predicate: predicate,
				Nodes:     childNodes,
			})
		}
	}
	if len(nd.Default) > 0 {
		var nodes []state.CommandNode
		for _, child := range nd.Default {
			if cn := wp.nodeDefToCommandNode(child); cn != nil {
				nodes = append(nodes, cn)
			}
		}
		cbn.Default = nodes
	}
	return cbn
}

// ---------------------------------------------------------------------------
// WASM callback helpers
// ---------------------------------------------------------------------------

// callOptionsCallback invokes a plugin's dynamic options callback.
func (wp *WasmPlugin) callOptionsCallback(cbName string, ctx state.StepContext) []model.Option {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		UserID:   int64(ctx.UserID),
		Locale:   ctx.Locale,
		Params:   ctx.Params,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm options callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return nil
	}
	var resp wasmrt.StepCallbackResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		slog.Error("wasm options callback response parse failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return nil
	}
	if resp.Error != "" {
		slog.Error("wasm options callback returned error", "plugin", wp.meta.ID, "callback", cbName, "error", resp.Error)
		return nil
	}
	opts := make([]model.Option, len(resp.Options))
	for i, o := range resp.Options {
		opts[i] = model.Option{Label: o.Label, Value: o.Value}
	}
	return opts
}

// callValidateCallback invokes a plugin's custom validation callback.
func (wp *WasmPlugin) callValidateCallback(cbName string, inputText string) bool {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		Input:    inputText,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm validate callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true // safe default: accept input
	}
	var resp wasmrt.StepCallbackResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return true
	}
	if resp.Result != nil {
		return *resp.Result
	}
	return true
}

// callConditionCallback invokes a plugin's condition evaluation callback.
func (wp *WasmPlugin) callConditionCallback(cbName string, params model.OptionMap) bool {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		Params:   params,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm condition callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true // safe default: show the step
	}
	var resp wasmrt.StepCallbackResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return true
	}
	if resp.Result != nil {
		return *resp.Result
	}
	return true
}

// callPaginationCallback invokes a plugin's pagination provider callback.
func (wp *WasmPlugin) callPaginationCallback(cbName string, ctx state.StepContext, page int) state.OptionsPage {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		UserID:   int64(ctx.UserID),
		Locale:   ctx.Locale,
		Params:   ctx.Params,
		Page:     page,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm pagination callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return state.OptionsPage{}
	}
	var resp wasmrt.StepCallbackResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		slog.Error("wasm pagination callback response parse failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return state.OptionsPage{}
	}
	if resp.Error != "" {
		slog.Error("wasm pagination callback returned error", "plugin", wp.meta.ID, "callback", cbName, "error", resp.Error)
		return state.OptionsPage{}
	}
	opts := make([]model.Option, len(resp.Options))
	for i, o := range resp.Options {
		opts[i] = model.Option{Label: o.Label, Value: o.Value}
	}
	return state.OptionsPage{Options: opts, HasMore: resp.HasMore}
}

// ---------------------------------------------------------------------------
// Declarative condition evaluation
// ---------------------------------------------------------------------------

// evalCondition evaluates a declarative condition against the collected params.
func evalCondition(cond *wasmrt.ConditionDef, params model.OptionMap) bool {
	if cond == nil {
		return true
	}

	// Compound conditions.
	if len(cond.And) > 0 {
		for _, c := range cond.And {
			if !evalCondition(c, params) {
				return false
			}
		}
		return true
	}
	if len(cond.Or) > 0 {
		for _, c := range cond.Or {
			if evalCondition(c, params) {
				return true
			}
		}
		return false
	}
	if cond.Not != nil {
		return !evalCondition(cond.Not, params)
	}

	// Simple conditions.
	val := params.Get(cond.Param)

	if cond.Set != nil {
		_, exists := params[cond.Param]
		if *cond.Set {
			return exists
		}
		return !exists
	}
	if cond.Eq != nil {
		return val == *cond.Eq
	}
	if cond.Neq != nil {
		return val != *cond.Neq
	}
	if cond.Match != "" {
		re, err := regexp.Compile(cond.Match)
		if err != nil {
			return true
		}
		return re.MatchString(val)
	}

	return true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseTextStyle converts a style string to model.TextStyle.
func parseTextStyle(s string) model.TextStyle {
	switch s {
	case "header":
		return model.StyleHeader
	case "subheader":
		return model.StyleSubheader
	case "code":
		return model.StyleCode
	case "quote":
		return model.StyleQuote
	default:
		return model.StylePlain
	}
}

// HandleCommand serializes the request, runs a one-shot handle_command action,
// and returns the result.
func (wp *WasmPlugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("wasm plugin %q command %q: marshal request: %w", wp.meta.ID, req.CommandName, err)
	}

	result, err := wp.compiled.CallHandleCommand(ctx, reqJSON, wp.config)
	if err != nil {
		return fmt.Errorf("wasm plugin %q command %q: handle_command: %w", wp.meta.ID, req.CommandName, err)
	}

	if len(result) > 0 {
		var resp struct {
			Error string `json:"error"`
			Reply string `json:"reply"`
			Logs  []struct {
				Level string `json:"level"`
				Msg   string `json:"msg"`
			} `json:"logs"`
			Messages []struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			} `json:"messages"`
		}
		if json.Unmarshal(result, &resp) == nil {
			for _, l := range resp.Logs {
				if l.Level == "error" {
					slog.Error("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
				} else {
					slog.Info("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
				}
			}

			if resp.Error != "" {
				return fmt.Errorf("wasm plugin %q command %q: %s", wp.meta.ID, req.CommandName, resp.Error)
			}

			if resp.Reply != "" && wp.reply != nil {
				if err := wp.reply(ctx, req, resp.Reply); err != nil {
					return fmt.Errorf("wasm plugin %q command %q: send reply: %w", wp.meta.ID, req.CommandName, err)
				}
			}

			if wp.send != nil {
				for _, m := range resp.Messages {
					if err := wp.send(ctx, req.ChannelType, m.ChatID, m.Text); err != nil {
						slog.Error("wasm plugin send failed", "plugin", wp.meta.ID, "chat_id", m.ChatID, "error", err)
					}
				}
			}
		}
	}

	slog.Debug("wasm: handle_command completed", "plugin", wp.meta.ID, "command", req.CommandName)
	return nil
}

// SetConfig updates the in-memory plugin configuration.
func (wp *WasmPlugin) SetConfig(config json.RawMessage) {
	wp.config = config
}

// Meta returns the cached plugin metadata.
func (wp *WasmPlugin) Meta() wasmrt.PluginMeta {
	return wp.meta
}

// IsWasm returns true, indicating this is a Wasm plugin.
func (wp *WasmPlugin) IsWasm() bool {
	return true
}

// Close releases the compiled module resources.
func (wp *WasmPlugin) Close(ctx context.Context) error {
	return wp.compiled.Close(ctx)
}
