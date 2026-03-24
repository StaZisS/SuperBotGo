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

var _ plugin.Plugin = (*WasmPlugin)(nil)

type SendFunc func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error

type WasmPlugin struct {
	compiled *wasmrt.CompiledModule
	meta     wasmrt.PluginMeta
	config   json.RawMessage
	send     SendFunc
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
			for _, nd := range cmd.Nodes {
				if cn := wp.nodeDefToCommandNode(nd); cn != nil {
					def.Nodes = append(def.Nodes, cn)
				}
			}
		} else {
			for _, step := range cmd.Steps {
				def.Nodes = append(def.Nodes, stepDefToNode(step))
			}
		}

		defs[i] = def
	}
	return defs
}

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
		node.MessageBuilder = func(_ state.StepContext) model.Message {
			return model.Message{}
		}
	}

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

func (wp *WasmPlugin) callValidateCallback(cbName string, inputText string) bool {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		Input:    inputText,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm validate callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true
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

func (wp *WasmPlugin) callConditionCallback(cbName string, params model.OptionMap) bool {
	req := wasmrt.StepCallbackRequest{
		Callback: cbName,
		Params:   params,
	}
	reqJSON, _ := json.Marshal(req)
	result, err := wp.compiled.CallStepCallback(context.Background(), reqJSON, wp.config)
	if err != nil {
		slog.Error("wasm condition callback failed", "plugin", wp.meta.ID, "callback", cbName, "error", err)
		return true
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
			if resp.Reply != "" && event.TriggerType == model.TriggerMessenger {
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

			var triggerChannelType model.ChannelType
			if event.TriggerType == model.TriggerMessenger {
				if m, mErr := event.Messenger(); mErr == nil {
					triggerChannelType = m.ChannelType
				}
			}

			for _, m := range resp.Messages {
				chType := m.ChannelType
				if chType == "" {
					chType = triggerChannelType
				}
				if chType == "" {
					slog.Error("wasm plugin send skipped: no channel type",
						"plugin", wp.meta.ID,
						"chat_id", m.ChatID)
					continue
				}
				if sendErr := wp.send(ctx, chType, m.ChatID, m.Text); sendErr != nil {
					slog.Error("wasm plugin send failed",
						"plugin", wp.meta.ID,
						"channel_type", chType,
						"chat_id", m.ChatID,
						"error", sendErr)
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
