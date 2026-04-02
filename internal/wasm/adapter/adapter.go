package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

var _ plugin.Plugin = (*WasmPlugin)(nil)

type SendFunc func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error

// LocalizedSendFunc handles delivery of messages that carry a locale→text map.
// The implementation resolves the target locale (from user or chat settings)
// and sends the appropriate text.
type LocalizedSendFunc func(ctx context.Context, msg model.MessageEntry, fallbackChannelType model.ChannelType) error

type WasmPlugin struct {
	compiled      *wasmrt.CompiledModule
	meta          wasmrt.PluginMeta
	config        json.RawMessage
	send          SendFunc
	localizedSend LocalizedSendFunc
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

		for _, nd := range t.Nodes {
			if cn := wp.nodeDefToCommandNode(nd); cn != nil {
				def.Nodes = append(def.Nodes, cn)
			}
		}

		defs = append(defs, def)
	}
	return defs
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

	if len(nd.Blocks) > 0 {
		blocks := nd.Blocks
		node.MessageBuilder = func(ctx state.StepContext) model.Message {
			var contentBlocks []model.ContentBlock
			for _, b := range blocks {
				switch b.Type {
				case "text":
					text := resolveLocalized(b.Text, b.Texts, ctx.Locale)
					contentBlocks = append(contentBlocks, model.TextBlock{
						Text:  text,
						Style: parseTextStyle(b.Style),
					})
				case "options":
					opts := make([]model.Option, len(b.Options))
					for j, o := range b.Options {
						label := resolveLocalized(o.Label, o.Labels, ctx.Locale)
						opts[j] = model.Option{Label: label, Value: o.Value}
					}
					prompt := resolveLocalized(b.Prompt, b.Prompts, ctx.Locale)
					contentBlocks = append(contentBlocks, model.OptionsBlock{
						Prompt:  prompt,
						Options: opts,
					})
				case "dynamic_options":
					opts := wp.callOptionsCallback(b.OptionsFn, ctx)
					prompt := resolveLocalized(b.Prompt, b.Prompts, ctx.Locale)
					contentBlocks = append(contentBlocks, model.OptionsBlock{
						Prompt:  prompt,
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
		node.MessageBuilder = func(_ state.StepContext) model.Message {
			return model.Message{}
		}
	}

	if nd.Validation != "" {
		pattern := nd.Validation
		node.Validate = func(input model.UserInput) bool {
			re, err := regexp.Compile(pattern)
			if err != nil {
				slog.Warn("wasm: invalid validation regex", "pattern", pattern, "error", err)
				return false
			}
			return re.MatchString(input.TextValue())
		}
	}
	if nd.ValidateFn != "" {
		cbName := nd.ValidateFn
		node.Validate = func(input model.UserInput) bool {
			return wp.callValidateCallback(cbName, input.TextValue())
		}
	}

	if nd.VisibleWhen != nil {
		cond := nd.VisibleWhen
		node.Condition = func(params model.OptionMap) bool {
			return evalCondition(cond, params)
		}
	}
	if nd.ConditionFn != "" {
		cbName := nd.ConditionFn
		node.Condition = func(params model.OptionMap) bool {
			return wp.callConditionCallback(cbName, params)
		}
	}

	if nd.Pagination != nil {
		pag := nd.Pagination
		cbName := pag.Provider
		node.Pagination = &state.PaginationConfig{
			Prompt:   pag.Prompt,
			Prompts:  pag.Prompts,
			PageSize: pag.PageSize,
			PageProvider: func(ctx state.StepContext, page int) state.OptionsPage {
				return wp.callPaginationCallback(cbName, ctx, page)
			},
		}
	}

	return node
}

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

// callStepCallback is the shared call/unmarshal path for all wasm step callbacks.
func (wp *WasmPlugin) callStepCallback(cbName string, req wasmrt.StepCallbackRequest) (*wasmrt.StepCallbackResponse, error) {
	req.Callback = cbName
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
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
	resp, err := wp.callStepCallback(cbName, wasmrt.StepCallbackRequest{
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

func (wp *WasmPlugin) callValidateCallback(cbName string, inputText string) bool {
	resp, err := wp.callStepCallback(cbName, wasmrt.StepCallbackRequest{Input: inputText})
	if err != nil {
		slog.Error("wasm validate callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true
	}
	if resp.Result != nil {
		return *resp.Result
	}
	return true
}

func (wp *WasmPlugin) callConditionCallback(cbName string, params model.OptionMap) bool {
	resp, err := wp.callStepCallback(cbName, wasmrt.StepCallbackRequest{Params: params})
	if err != nil {
		slog.Error("wasm condition callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true
	}
	if resp.Result != nil {
		return *resp.Result
	}
	return true
}

func (wp *WasmPlugin) callPaginationCallback(cbName string, ctx state.StepContext, page int) state.OptionsPage {
	resp, err := wp.callStepCallback(cbName, wasmrt.StepCallbackRequest{
		UserID: int64(ctx.UserID),
		Locale: ctx.Locale,
		Params: ctx.Params,
		Page:   page,
	})
	if err != nil {
		slog.Error("wasm pagination callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return state.OptionsPage{}
	}
	return state.OptionsPage{Options: convertOptions(resp.Options, ctx.Locale), HasMore: resp.HasMore}
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

	result, err := wp.compiled.CallHandleEvent(ctx, eventJSON, wp.config)
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

		if wp.send != nil {
			// Localized reply — resolve locale from chat/user and send.
			if len(resp.ReplyTexts) > 0 && event.TriggerType == model.TriggerMessenger {
				if m, mErr := event.Messenger(); mErr == nil {
					if wp.localizedSend != nil {
						entry := model.MessageEntry{
							ChatID: m.ChatID,
							Texts:  resp.ReplyTexts,
						}
						if sendErr := wp.localizedSend(ctx, entry, m.ChannelType); sendErr != nil {
							slog.Error("wasm plugin localized reply failed",
								"plugin", wp.meta.ID,
								"chat_id", m.ChatID,
								"error", sendErr)
							return &resp, fmt.Errorf("wasm plugin %q localized reply send: %w", wp.meta.ID, sendErr)
						}
					} else {
						// Fallback: resolve to default locale.
						text := ResolveLocalizedText(resp.ReplyTexts, locale.Default())
						if sendErr := wp.send(ctx, m.ChannelType, m.ChatID, text); sendErr != nil {
							slog.Error("wasm plugin reply failed",
								"plugin", wp.meta.ID,
								"chat_id", m.ChatID,
								"error", sendErr)
							return &resp, fmt.Errorf("wasm plugin %q reply send: %w", wp.meta.ID, sendErr)
						}
					}
				}
			} else if resp.Reply != "" && event.TriggerType == model.TriggerMessenger {
				if m, mErr := event.Messenger(); mErr == nil {
					if sendErr := wp.send(ctx, m.ChannelType, m.ChatID, resp.Reply); sendErr != nil {
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
	}

	return &resp, nil
}

func (wp *WasmPlugin) Triggers() []wasmrt.TriggerDef {
	return wp.meta.Triggers
}

func (wp *WasmPlugin) SetConfig(config json.RawMessage) {
	wp.config = config
}

func (wp *WasmPlugin) Meta() wasmrt.PluginMeta {
	return wp.meta
}

func (wp *WasmPlugin) Close(ctx context.Context) error {
	return wp.compiled.Close(ctx)
}
