package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

var _ plugin.Plugin = (*WasmPlugin)(nil)

// MessageSendFunc sends a full Message to a chat.
type MessageSendFunc func(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error

type WasmPlugin struct {
	compiled    *wasmrt.CompiledModule
	meta        wasmrt.PluginMeta
	configMu    sync.RWMutex
	config      json.RawMessage
	messageSend MessageSendFunc
}

func (wp *WasmPlugin) ID() string {
	return wp.meta.ID
}

func (wp *WasmPlugin) Name() string {
	return wp.meta.Name
}

func (wp *WasmPlugin) Version() string {
	return wp.meta.Version
}

func (wp *WasmPlugin) Commands() []*state.CommandDefinition {
	var defs []*state.CommandDefinition
	for _, t := range wp.meta.Triggers {
		if t.Type != "messenger" {
			continue
		}
		def := &state.CommandDefinition{
			Name:        t.Name,
			Description: t.Description,
		}

		def.Nodes = wp.commandNodes(t.Nodes)

		defs = append(defs, def)
	}
	return defs
}

func (wp *WasmPlugin) commandNodes(defs []wasmrt.NodeDef) []state.CommandNode {
	nodes := make([]state.CommandNode, 0, len(defs))
	for _, nd := range defs {
		if cn := wp.nodeDefToCommandNode(nd); cn != nil {
			nodes = append(nodes, cn)
		}
	}
	return nodes
}

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

func (wp *WasmPlugin) stepNodeDefToStepNode(nd wasmrt.NodeDef) state.StepNode {
	node := state.StepNode{
		ParamName: nd.Param,
	}

	node.MessageBuilder = wp.buildStepMessage(nd.Blocks)
	node.ValidateWithContext = wp.buildStepValidator(nd)
	node.ConditionWithContext = wp.buildStepCondition(nd)
	node.Pagination = wp.buildStepPagination(nd)

	return node
}

func (wp *WasmPlugin) branchNodeDefToBranchNode(nd wasmrt.NodeDef) state.BranchNode {
	bn := state.BranchNode{
		OnParam: nd.OnParam,
		Cases:   make(map[string][]state.CommandNode),
	}
	for value, children := range nd.Cases {
		bn.Cases[value] = wp.commandNodes(children)
	}
	if len(nd.Default) > 0 {
		bn.Default = wp.commandNodes(nd.Default)
	}
	return bn
}

func (wp *WasmPlugin) condBranchNodeDefToCondBranchNode(nd wasmrt.NodeDef) state.ConditionalBranchNode {
	cbn := state.ConditionalBranchNode{}
	for _, cc := range nd.ConditionalCases {
		predicate := wp.conditionalPredicate(cc.Condition, cc.ConditionFn)

		if predicate != nil {
			ccase := state.ConditionalCase{
				Predicate: func(params model.OptionMap) bool {
					return predicate(params)
				},
				Nodes: wp.commandNodes(cc.Nodes),
			}
			if cc.ConditionFn != "" {
				callbackName := cc.ConditionFn
				ccase.PredicateWithContext = func(ctx state.StepContext) bool {
					return wp.callConditionCallback(ctx, callbackName, ctx.Params)
				}
			}
			cbn.Cases = append(cbn.Cases, ccase)
		}
	}
	if len(nd.Default) > 0 {
		cbn.Default = wp.commandNodes(nd.Default)
	}
	return cbn
}

func (wp *WasmPlugin) buildStepMessage(blocks []wasmrt.BlockDef) func(state.StepContext) model.Message {
	if len(blocks) == 0 {
		return func(state.StepContext) model.Message { return model.Message{} }
	}
	return func(ctx state.StepContext) model.Message {
		contentBlocks := make([]model.ContentBlock, 0, len(blocks))
		for _, block := range blocks {
			if rendered := wp.renderStepBlock(block, ctx); rendered != nil {
				contentBlocks = append(contentBlocks, rendered)
			}
		}
		return model.Message{Blocks: contentBlocks}
	}
}

func (wp *WasmPlugin) renderStepBlock(block wasmrt.BlockDef, ctx state.StepContext) model.ContentBlock {
	switch block.Type {
	case "text":
		return model.TextBlock{
			Text:  resolveLocalized(block.Text, block.Texts, ctx.Locale),
			Style: parseTextStyle(block.Style),
		}
	case "options":
		return model.OptionsBlock{
			Prompt:  resolveLocalized(block.Prompt, block.Prompts, ctx.Locale),
			Options: convertOptions(block.Options, ctx.Locale),
		}
	case "dynamic_options":
		return model.OptionsBlock{
			Prompt:  resolveLocalized(block.Prompt, block.Prompts, ctx.Locale),
			Options: wp.callOptionsCallback(block.OptionsFn, ctx),
		}
	case "link":
		return model.LinkBlock{URL: block.URL, Label: block.Label}
	case "image":
		return model.ImageBlock{URL: block.URL}
	default:
		return nil
	}
}

func (wp *WasmPlugin) buildStepValidator(nd wasmrt.NodeDef) func(state.StepContext, model.UserInput) bool {
	if nd.ValidateFn != "" {
		cbName := nd.ValidateFn
		return func(ctx state.StepContext, input model.UserInput) bool {
			return wp.callValidateCallback(ctx, cbName, input.TextValue())
		}
	}
	if nd.Validation == "" {
		return nil
	}
	pattern := nd.Validation
	return func(_ state.StepContext, input model.UserInput) bool {
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn("wasm: invalid validation regex", "pattern", pattern, "error", err)
			return false
		}
		return re.MatchString(input.TextValue())
	}
}

func (wp *WasmPlugin) buildStepCondition(nd wasmrt.NodeDef) func(state.StepContext) bool {
	if nd.ConditionFn != "" {
		cbName := nd.ConditionFn
		return func(ctx state.StepContext) bool {
			return wp.callConditionCallback(ctx, cbName, ctx.Params)
		}
	}
	if nd.VisibleWhen == nil {
		return nil
	}
	cond := nd.VisibleWhen
	return func(ctx state.StepContext) bool {
		return evalCondition(cond, ctx.Params)
	}
}

func (wp *WasmPlugin) buildStepPagination(nd wasmrt.NodeDef) *state.PaginationConfig {
	if nd.Pagination == nil {
		return nil
	}
	pag := nd.Pagination
	cbName := pag.Provider
	return &state.PaginationConfig{
		Prompt:   pag.Prompt,
		Prompts:  pag.Prompts,
		PageSize: pag.PageSize,
		PageProvider: func(ctx state.StepContext, page int) state.OptionsPage {
			return wp.callPaginationCallback(cbName, ctx, page)
		},
	}
}

func (wp *WasmPlugin) conditionalPredicate(cond *wasmrt.ConditionDef, callbackName string) func(model.OptionMap) bool {
	if callbackName != "" {
		return func(params model.OptionMap) bool {
			return wp.callConditionCallback(state.StepContext{Params: params}, callbackName, params)
		}
	}
	if cond == nil {
		return nil
	}
	return func(params model.OptionMap) bool {
		return evalCondition(cond, params)
	}
}

// callStepCallback is the shared call/unmarshal path for all wasm step callbacks.
func (wp *WasmPlugin) callStepCallback(ctx context.Context, cbName string, req wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error) {
	req.Callback = cbName
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := wp.compiled.CallStepCallback(ctx, reqJSON, wp.Config())
	if err != nil {
		return nil, err
	}
	var resp wasmrt.StepCallbackResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}
	return &resp, nil
}

func convertOptions(defs []wasmrt.OptionDef, locale string) []model.Option {
	opts := make([]model.Option, len(defs))
	for i, o := range defs {
		label := resolveLocalized(o.Label, o.Labels, locale)
		opts[i] = model.Option{Label: label, Value: o.Value}
	}
	return opts
}

func (wp *WasmPlugin) callOptionsCallback(cbName string, ctx state.StepContext) []model.Option {
	resp, err := wp.callStepCallback(ctx.Context, cbName, wasmrt.StepCallbackRequest{
		UserID: int64(ctx.UserID),
		Locale: ctx.Locale,
		Params: ctx.Params,
	})
	if err != nil {
		slog.Error("wasm options callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return nil
	}
	return convertOptions(resp.Options, ctx.Locale)
}

func (wp *WasmPlugin) callValidateCallback(ctx state.StepContext, cbName string, inputText string) bool {
	resp, err := wp.callStepCallback(ctx.Context, cbName, wasmrt.StepCallbackRequest{
		UserID: int64(ctx.UserID),
		Locale: ctx.Locale,
		Params: ctx.Params,
		Input:  inputText,
	})
	if err != nil {
		slog.Error("wasm validate callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return false
	}
	if resp.Result != nil {
		return *resp.Result
	}
	slog.Error("wasm validate callback returned empty result", "plugin", wp.meta.ID, "callback", cbName)
	return false
}

func (wp *WasmPlugin) callConditionCallback(ctx state.StepContext, cbName string, params model.OptionMap) bool {
	resp, err := wp.callStepCallback(ctx.Context, cbName, wasmrt.StepCallbackRequest{
		UserID: int64(ctx.UserID),
		Locale: ctx.Locale,
		Params: params,
	})
	if err != nil {
		slog.Error("wasm condition callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return false
	}
	if resp.Result != nil {
		return *resp.Result
	}
	slog.Error("wasm condition callback returned empty result", "plugin", wp.meta.ID, "callback", cbName)
	return false
}

func (wp *WasmPlugin) callPaginationCallback(cbName string, ctx state.StepContext, page int) state.OptionsPage {
	resp, err := wp.callStepCallback(ctx.Context, cbName, wasmrt.StepCallbackRequest{
		UserID: int64(ctx.UserID),
		Locale: ctx.Locale,
		Params: ctx.Params,
		Page:   page,
	})
	if err != nil {
		slog.Error("wasm pagination callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return state.OptionsPage{Error: paginationErrorMessage(ctx.Locale)}
	}
	if strings.TrimSpace(resp.Error) != "" {
		slog.Error("wasm pagination callback returned plugin error", "plugin", wp.meta.ID, "callback", cbName, "error", resp.Error)
		return state.OptionsPage{Error: paginationErrorMessage(ctx.Locale)}
	}
	return state.OptionsPage{Options: convertOptions(resp.Options, ctx.Locale), HasMore: resp.HasMore}
}

func paginationErrorMessage(locale string) string {
	if strings.HasPrefix(locale, "ru") {
		return "Не удалось загрузить варианты. Попробуйте ещё раз."
	}
	return "Failed to load options. Please try again."
}

func evalCondition(cond *wasmrt.ConditionDef, params model.OptionMap) bool {
	if cond == nil {
		return true
	}

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

// resolveLocalized returns the locale-specific text from the texts map,
// falling back to the single-string fallback if the map is empty.
func resolveLocalized(fallback string, texts map[string]string, locale string) string {
	if len(texts) > 0 {
		if resolved := ResolveLocalizedText(texts, locale); resolved != "" {
			return resolved
		}
	}
	return fallback
}

// replyBlocksToMessage converts reply blocks from the plugin response
// into a model.Message, resolving localized text per the given locale.
func replyBlocksToMessage(blocks []model.ReplyBlock, loc string) model.Message {
	content := make([]model.ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			text := resolveLocalized(b.Text, b.Texts, loc)
			content = append(content, model.TextBlock{Text: text, Style: parseTextStyle(b.Style)})
		case "mention":
			content = append(content, model.MentionBlock{UserID: b.UserID})
		case "file":
			content = append(content, model.FileBlock{
				FileRef: model.FileRef{ID: b.FileID},
				Caption: b.Caption,
			})
		case "link":
			content = append(content, model.LinkBlock{URL: b.URL, Label: b.Label})
		case "image":
			content = append(content, model.ImageBlock{URL: b.URL})
		}
	}
	return model.Message{Blocks: content}
}

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

func (wp *WasmPlugin) HandleEvent(ctx context.Context, event model.Event) (*model.EventResponse, error) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q: marshal event: %w", wp.meta.ID, err)
	}

	result, err := wp.compiled.CallHandleEvent(ctx, eventJSON, wp.Config())
	if err != nil {
		return nil, fmt.Errorf("wasm plugin %q handle_event: %w", wp.meta.ID, err)
	}

	var resp model.EventResponse
	if len(result) > 0 {
		if err := json.Unmarshal(result, &resp); err != nil {
			return nil, fmt.Errorf("wasm plugin %q handle_event: unmarshal response: %w", wp.meta.ID, err)
		}

		for _, l := range resp.Logs {
			if l.Level == "error" {
				slog.Error("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
			} else {
				slog.Info("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
			}
		}

		if len(resp.ReplyBlocks) > 0 && wp.messageSend != nil && event.TriggerType == model.TriggerMessenger {
			if m, mErr := event.Messenger(); mErr == nil {
				msg := replyBlocksToMessage(resp.ReplyBlocks, string(m.Locale))
				if sendErr := wp.messageSend(ctx, m.ChannelType, m.ChatID, msg); sendErr != nil {
					slog.Error("wasm plugin reply failed",
						"plugin", wp.meta.ID,
						"channel_type", m.ChannelType,
						"chat_id", m.ChatID,
						"error", sendErr)
					return &resp, fmt.Errorf("wasm plugin %q reply send: %w", wp.meta.ID, sendErr)
				}
			}
		}
	}

	return &resp, nil
}

func (wp *WasmPlugin) Triggers() []wasmrt.TriggerDef {
	return wp.meta.Triggers
}

func (wp *WasmPlugin) SetConfig(config json.RawMessage) {
	wp.configMu.Lock()
	defer wp.configMu.Unlock()
	wp.config = cloneRawMessage(config)
}

func (wp *WasmPlugin) Config() json.RawMessage {
	wp.configMu.RLock()
	defer wp.configMu.RUnlock()
	return cloneRawMessage(wp.config)
}

func (wp *WasmPlugin) Meta() wasmrt.PluginMeta {
	return wp.meta
}

func (wp *WasmPlugin) SupportsRPCMethod(method string) bool {
	for _, candidate := range wp.meta.RPCMethods {
		if candidate.Name == method {
			return true
		}
	}
	return false
}

func (wp *WasmPlugin) Close(ctx context.Context) error {
	return wp.compiled.Close(ctx)
}
