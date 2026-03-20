package state

import (
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/model"
)

// CommandBuilder constructs a CommandDefinition using method chaining.
type CommandBuilder struct {
	name         string
	description  string
	requirements *model.RoleRequirements
	nodes        []CommandNode
}

// NewCommand starts building a new command definition with the given name.
func NewCommand(name string) *CommandBuilder {
	return &CommandBuilder{name: name}
}

// Description sets the human-readable description for the command.
func (b *CommandBuilder) Description(d string) *CommandBuilder {
	b.description = d
	return b
}

// RequireRole sets role requirements for the command.
func (b *CommandBuilder) RequireRole(systemRole string, globalRoles []string) *CommandBuilder {
	b.requirements = &model.RoleRequirements{
		SystemRole:  systemRole,
		GlobalRoles: globalRoles,
	}
	return b
}

// Step adds a parameter-collection step to the command.
func (b *CommandBuilder) Step(paramName string, configure func(*StepBuilder)) *CommandBuilder {
	sb := &StepBuilder{paramName: paramName}
	configure(sb)
	b.nodes = append(b.nodes, sb.build())
	return b
}

// Branch adds a value-based branch node to the command.
func (b *CommandBuilder) Branch(onParam string, configure func(*BranchBuilder)) *CommandBuilder {
	bb := &BranchBuilder{onParam: onParam}
	configure(bb)
	b.nodes = append(b.nodes, bb.build())
	return b
}

// ConditionalBranch adds a predicate-based branch node to the command.
func (b *CommandBuilder) ConditionalBranch(configure func(*ConditionalBranchBuilder)) *CommandBuilder {
	cb := &ConditionalBranchBuilder{}
	configure(cb)
	b.nodes = append(b.nodes, cb.build())
	return b
}

// Build finalizes and returns the CommandDefinition.
func (b *CommandBuilder) Build() *CommandDefinition {
	return &CommandDefinition{
		Name:         b.name,
		Description:  b.description,
		Requirements: b.requirements,
		Nodes:        b.nodes,
	}
}

// StepBuilder configures a single parameter-collection step.
type StepBuilder struct {
	paramName      string
	validateFn     func(model.UserInput) bool
	condition      func(model.OptionMap) bool
	blockFactories []func(StepContext) model.ContentBlock
	paginationCfg  *PaginationConfig
}

// Validate sets a validation function for user input at this step.
func (s *StepBuilder) Validate(fn func(model.UserInput) bool) {
	s.validateFn = fn
}

// VisibleWhen sets a predicate that controls whether this step is active.
func (s *StepBuilder) VisibleWhen(fn func(model.OptionMap) bool) {
	s.condition = fn
}

// Prompt configures the message shown to the user at this step.
func (s *StepBuilder) Prompt(configure func(*PromptBuilder)) {
	pb := &PromptBuilder{}
	configure(pb)
	s.blockFactories = pb.blockFactories
	s.paginationCfg = pb.paginationCfg
}

func (s *StepBuilder) build() StepNode {
	factories := s.blockFactories
	var msgBuilder func(StepContext) model.Message
	if len(factories) > 0 {
		msgBuilder = func(ctx StepContext) model.Message {
			blocks := make([]model.ContentBlock, len(factories))
			for i, f := range factories {
				blocks[i] = f(ctx)
			}
			return model.Message{Blocks: blocks}
		}
	} else {
		msgBuilder = func(_ StepContext) model.Message {
			return model.Message{}
		}
	}

	return StepNode{
		ParamName:      s.paramName,
		MessageBuilder: msgBuilder,
		Validate:       s.validateFn,
		Condition:      s.condition,
		Pagination:     s.paginationCfg,
	}
}

// PromptBuilder constructs the message content blocks for a step prompt.
type PromptBuilder struct {
	blockFactories []func(StepContext) model.ContentBlock
	paginationCfg  *PaginationConfig
}

// Text adds a static text block.
func (p *PromptBuilder) Text(text string, style model.TextStyle) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.TextBlock{Text: text, Style: style}
	})
}

// LocalizedText adds a text block whose content is resolved via i18n at render
// time using the StepContext's locale.
func (p *PromptBuilder) LocalizedText(key string, style model.TextStyle) {
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.TextBlock{Text: i18n.Get(key, ctx.Locale), Style: style}
	})
}

// Options adds a static options block with the given prompt text.
func (p *PromptBuilder) Options(prompt string, configure func(*OptionsBuilder)) {
	ob := &OptionsBuilder{}
	configure(ob)
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.OptionsBlock{Prompt: prompt, Options: ob.resolve(ctx)}
	})
}

// LocalizedOptions adds an options block whose prompt is resolved via i18n.
func (p *PromptBuilder) LocalizedOptions(promptKey string, configure func(*OptionsBuilder)) {
	ob := &OptionsBuilder{}
	configure(ob)
	p.blockFactories = append(p.blockFactories, func(ctx StepContext) model.ContentBlock {
		return model.OptionsBlock{Prompt: i18n.Get(promptKey, ctx.Locale), Options: ob.resolve(ctx)}
	})
}

// PaginatedOptions adds a paginated option selection to the step.
// The provider function returns all options; they are sliced into pages of
// pageSize.
func (p *PromptBuilder) PaginatedOptions(prompt string, pageSize int, provider func(StepContext) []model.Option) {
	p.paginationCfg = &PaginationConfig{
		Prompt:   prompt,
		PageSize: pageSize,
		PageProvider: func(ctx StepContext, page int) OptionsPage {
			all := provider(ctx)
			start := page * pageSize
			if start >= len(all) {
				return OptionsPage{Options: nil, HasMore: false}
			}
			end := start + pageSize
			if end > len(all) {
				end = len(all)
			}
			return OptionsPage{
				Options: all[start:end],
				HasMore: end < len(all),
			}
		},
	}
}

// PaginatedOptionsWithProvider adds paginated options using a custom page
// provider that handles pagination externally.
func (p *PromptBuilder) PaginatedOptionsWithProvider(prompt string, pageSize int, provider func(StepContext, int) OptionsPage) {
	p.paginationCfg = &PaginationConfig{
		Prompt:       prompt,
		PageSize:     pageSize,
		PageProvider: provider,
	}
}

// Link adds a hyperlink block.
func (p *PromptBuilder) Link(url, label string) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.LinkBlock{URL: url, Label: label}
	})
}

// Image adds an image block.
func (p *PromptBuilder) Image(url string) {
	p.blockFactories = append(p.blockFactories, func(_ StepContext) model.ContentBlock {
		return model.ImageBlock{URL: url}
	})
}

// OptionsBuilder constructs a list of selectable options.
type OptionsBuilder struct {
	optionBuilders  []func(StepContext) model.Option
	dynamicProvider func(StepContext) []model.Option
}

// Add adds a static option with the given label and value.
func (o *OptionsBuilder) Add(label, value string) {
	o.optionBuilders = append(o.optionBuilders, func(_ StepContext) model.Option {
		return model.Option{Label: label, Value: value}
	})
}

// LocalizedOption adds an option whose label is resolved via i18n.
// The value is also passed as the first template argument (V0), so
// "schedule.building_option" with value "1" renders as "Building 1".
func (o *OptionsBuilder) LocalizedOption(key, value string) {
	o.optionBuilders = append(o.optionBuilders, func(ctx StepContext) model.Option {
		return model.Option{Label: i18n.Get(key, ctx.Locale, value), Value: value}
	})
}

// From sets a dynamic provider that supplies the full list of options at
// render time, replacing any statically-added options.
func (o *OptionsBuilder) From(provider func() []model.Option) {
	o.dynamicProvider = func(_ StepContext) []model.Option {
		return provider()
	}
}

// FromContext sets a dynamic provider that receives the StepContext.
func (o *OptionsBuilder) FromContext(provider func(StepContext) []model.Option) {
	o.dynamicProvider = provider
}

func (o *OptionsBuilder) resolve(ctx StepContext) []model.Option {
	if o.dynamicProvider != nil {
		return o.dynamicProvider(ctx)
	}
	result := make([]model.Option, len(o.optionBuilders))
	for i, builder := range o.optionBuilders {
		result[i] = builder(ctx)
	}
	return result
}

// BranchBuilder configures value-based branching on a previously collected
// parameter.
type BranchBuilder struct {
	onParam      string
	cases        map[string][]CommandNode
	defaultNodes []CommandNode
}

// Case adds a branch that is followed when the parameter equals value.
func (b *BranchBuilder) Case(value string, configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	if b.cases == nil {
		b.cases = make(map[string][]CommandNode)
	}
	b.cases[value] = nlb.nodes
}

// Default sets the fallback branch when no case matches.
func (b *BranchBuilder) Default(configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	b.defaultNodes = nlb.nodes
}

func (b *BranchBuilder) build() BranchNode {
	return BranchNode{
		OnParam: b.onParam,
		Cases:   b.cases,
		Default: b.defaultNodes,
	}
}

// ConditionalBranchBuilder configures predicate-based branching.
type ConditionalBranchBuilder struct {
	cases        []ConditionalCase
	defaultNodes []CommandNode
}

// Case adds a conditional branch with the given predicate.
func (c *ConditionalBranchBuilder) Case(predicate func(model.OptionMap) bool, configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	c.cases = append(c.cases, ConditionalCase{
		Predicate: predicate,
		Nodes:     nlb.nodes,
	})
}

// Default sets the fallback branch when no predicate matches.
func (c *ConditionalBranchBuilder) Default(configure func(*NodeListBuilder)) {
	nlb := &NodeListBuilder{}
	configure(nlb)
	c.defaultNodes = nlb.nodes
}

func (c *ConditionalBranchBuilder) build() ConditionalBranchNode {
	return ConditionalBranchNode{
		Cases:   c.cases,
		Default: c.defaultNodes,
	}
}

// NodeListBuilder is used inside branches to add nested steps and sub-branches.
type NodeListBuilder struct {
	nodes []CommandNode
}

// Step adds a parameter-collection step.
func (n *NodeListBuilder) Step(paramName string, configure func(*StepBuilder)) {
	sb := &StepBuilder{paramName: paramName}
	configure(sb)
	n.nodes = append(n.nodes, sb.build())
}

// Branch adds a value-based branch node.
func (n *NodeListBuilder) Branch(onParam string, configure func(*BranchBuilder)) {
	bb := &BranchBuilder{onParam: onParam}
	configure(bb)
	n.nodes = append(n.nodes, bb.build())
}

// ConditionalBranch adds a predicate-based branch node.
func (n *NodeListBuilder) ConditionalBranch(configure func(*ConditionalBranchBuilder)) {
	cb := &ConditionalBranchBuilder{}
	configure(cb)
	n.nodes = append(n.nodes, cb.build())
}
