package wasmplugin

import "strconv"

// callbackMap is a registry of callback functions keyed by auto-generated names.
// Values are one of:
//   - func(*CallbackContext) bool       — validate / condition
//   - func(*CallbackContext) []Option   — dynamic options
//   - func(*CallbackContext) OptionsPage — paginated options
type callbackMap = map[string]interface{}

// ---------- Text style ----------

// TextStyle controls how a text block is rendered by the platform adapter.
type TextStyle string

const (
	StylePlain     TextStyle = "plain"
	StyleHeader    TextStyle = "header"
	StyleSubheader TextStyle = "subheader"
	StyleCode      TextStyle = "code"
	StyleQuote     TextStyle = "quote"
)

// ---------- Callback context ----------

// CallbackContext holds contextual information available to step callback
// functions (validation, dynamic options, pagination, conditions).
type CallbackContext struct {
	UserID int64
	Locale string
	Params map[string]string
	Page   int    // current page (pagination providers)
	Input  string // user input text (validators)
	config map[string]interface{}
}

// Config returns a config value by key, or the fallback if not set.
func (c *CallbackContext) Config(key, fallback string) string {
	if c.config == nil {
		return fallback
	}
	v, ok := c.config[key]
	if !ok {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

// ---------- OptionsPage ----------

// OptionsPage holds a page of options and whether more pages exist.
type OptionsPage struct {
	Options []Option
	HasMore bool
}

// ---------- Helpers ----------

// Opt is a convenience constructor for Option.
func Opt(label, value string) Option {
	return Option{Label: label, Value: value}
}

// ---------- Node interface ----------

// Node is the interface for all command flow nodes.
// Implemented by *StepBuilder, *branchNode, and *conditionalBranchNode.
type Node interface {
	// toNodeDef converts the node to an internal nodeDef for JSON serialization,
	// registering any callback functions in reg as a side-effect.
	toNodeDef(cmdName string, reg callbackMap) nodeDef
}

// ---------- Step builder ----------

type block struct {
	typ       string // "text", "options", "dynamic_options", "link", "image"
	texts     map[string]string
	style     TextStyle
	prompts   map[string]string
	options   []Option
	optionsFn func(ctx *CallbackContext) []Option
	url       string
	label     string
}

type paginationCfg struct {
	prompts  map[string]string
	pageSize int
	provider func(ctx *CallbackContext) OptionsPage
}

// StepBuilder constructs a step node via method chaining. Create with [NewStep].
type StepBuilder struct {
	param       string
	blocks      []block
	validation  string
	validateFn  func(ctx *CallbackContext) bool
	visibleWhen *Condition
	conditionFn func(ctx *CallbackContext) bool
	pagination  *paginationCfg
}

// NewStep starts building a parameter-collection step.
func NewStep(param string) *StepBuilder {
	return &StepBuilder{param: param}
}

// Text adds a styled text block.
func (s *StepBuilder) Text(text string, style TextStyle) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "text", texts: map[string]string{"en": text}, style: style})
	return s
}

// Options adds a static options block.
func (s *StepBuilder) Options(prompt string, options ...Option) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "options", prompts: map[string]string{"en": prompt}, options: options})
	return s
}

// DynamicOptions adds options resolved at runtime via a WASM callback.
// The provider function is called each time the step is displayed.
func (s *StepBuilder) DynamicOptions(prompt string, provider func(ctx *CallbackContext) []Option) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "dynamic_options", prompts: map[string]string{"en": prompt}, optionsFn: provider})
	return s
}

// PaginatedOptions adds paginated option selection. The provider is called
// each time a page needs to be displayed.
func (s *StepBuilder) PaginatedOptions(prompt string, pageSize int, provider func(ctx *CallbackContext) OptionsPage) *StepBuilder {
	s.pagination = &paginationCfg{prompts: map[string]string{"en": prompt}, pageSize: pageSize, provider: provider}
	return s
}

// LocalizedText adds a text block with locale-specific content.
// The texts map is keyed by locale code (e.g. "en", "ru").
func (s *StepBuilder) LocalizedText(texts map[string]string, style TextStyle) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "text", texts: texts, style: style})
	return s
}

// LocalizedOptions adds static options with a localized prompt.
func (s *StepBuilder) LocalizedOptions(prompts map[string]string, options ...Option) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "options", prompts: prompts, options: options})
	return s
}

// LocalizedDynamicOptions adds dynamic options with a localized prompt.
func (s *StepBuilder) LocalizedDynamicOptions(prompts map[string]string, provider func(ctx *CallbackContext) []Option) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "dynamic_options", prompts: prompts, optionsFn: provider})
	return s
}

// LocalizedPaginatedOptions adds paginated options with a localized prompt.
func (s *StepBuilder) LocalizedPaginatedOptions(prompts map[string]string, pageSize int, provider func(ctx *CallbackContext) OptionsPage) *StepBuilder {
	s.pagination = &paginationCfg{prompts: prompts, pageSize: pageSize, provider: provider}
	return s
}

// Link adds a hyperlink block.
func (s *StepBuilder) Link(url, label string) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "link", url: url, label: label})
	return s
}

// Image adds an image block.
func (s *StepBuilder) Image(url string) *StepBuilder {
	s.blocks = append(s.blocks, block{typ: "image", url: url})
	return s
}

// Validate sets a regex validation pattern for user input.
func (s *StepBuilder) Validate(pattern string) *StepBuilder {
	s.validation = pattern
	return s
}

// ValidateFunc sets a custom validation function invoked via WASM callback.
// Takes precedence over Validate (regex) if both are set.
func (s *StepBuilder) ValidateFunc(fn func(ctx *CallbackContext) bool) *StepBuilder {
	s.validateFn = fn
	return s
}

// VisibleWhen sets a declarative condition for step visibility.
// The condition is evaluated on the host side without a WASM call.
func (s *StepBuilder) VisibleWhen(cond *Condition) *StepBuilder {
	s.visibleWhen = cond
	return s
}

// VisibleWhenFunc sets a callback-based condition for step visibility.
// Takes precedence over VisibleWhen (declarative) if both are set.
func (s *StepBuilder) VisibleWhenFunc(fn func(ctx *CallbackContext) bool) *StepBuilder {
	s.conditionFn = fn
	return s
}

func (s *StepBuilder) toNodeDef(cmdName string, reg callbackMap) nodeDef {
	nd := nodeDef{
		Type:       "step",
		Param:      s.param,
		Validation: s.validation,
	}

	for _, b := range s.blocks {
		bd := blockDef{Type: b.typ}
		switch b.typ {
		case "text":
			bd.Texts = b.texts
			bd.Style = string(b.style)
		case "options":
			bd.Prompts = b.prompts
			for _, o := range b.options {
				bd.Options = append(bd.Options, optionDef{Label: o.Label, Value: o.Value})
			}
		case "dynamic_options":
			bd.Prompts = b.prompts
			if b.optionsFn != nil {
				cbName := cmdName + ":options:" + s.param
				reg[cbName] = b.optionsFn
				bd.OptionsFn = cbName
			}
		case "link":
			bd.URL = b.url
			bd.Label = b.label
		case "image":
			bd.URL = b.url
		}
		nd.Blocks = append(nd.Blocks, bd)
	}

	if s.validateFn != nil {
		cbName := cmdName + ":validate:" + s.param
		reg[cbName] = s.validateFn
		nd.ValidateFn = cbName
	}

	if s.visibleWhen != nil {
		nd.VisibleWhen = s.visibleWhen.toCondDef()
	}
	if s.conditionFn != nil {
		cbName := cmdName + ":condition:" + s.param
		reg[cbName] = s.conditionFn
		nd.ConditionFn = cbName
	}

	if s.pagination != nil {
		cbName := cmdName + ":paginate:" + s.param
		reg[cbName] = s.pagination.provider
		nd.Pagination = &paginationDef{
			Prompts:  s.pagination.prompts,
			PageSize: s.pagination.pageSize,
			Provider: cbName,
		}
	}

	return nd
}

// ---------- Branch (value-based) ----------

// BranchCase is one arm of a value-based branch.
type BranchCase struct {
	value     string
	nodes     []Node
	isDefault bool
}

// Case creates a branch case matching when the parameter equals value.
func Case(value string, nodes ...Node) BranchCase {
	return BranchCase{value: value, nodes: nodes}
}

// DefaultCase creates the fallback branch when no case matches.
func DefaultCase(nodes ...Node) BranchCase {
	return BranchCase{isDefault: true, nodes: nodes}
}

type branchNode struct {
	onParam string
	cases   []BranchCase
}

// BranchOn creates a value-based branch on a previously collected parameter.
func BranchOn(param string, cases ...BranchCase) Node {
	return &branchNode{onParam: param, cases: cases}
}

func (b *branchNode) toNodeDef(cmdName string, reg callbackMap) nodeDef {
	nd := nodeDef{
		Type:    "branch",
		OnParam: b.onParam,
		Cases:   make(map[string][]nodeDef),
	}
	for _, c := range b.cases {
		children := make([]nodeDef, len(c.nodes))
		for i, n := range c.nodes {
			children[i] = n.toNodeDef(cmdName, reg)
		}
		if c.isDefault {
			nd.Default = children
		} else {
			nd.Cases[c.value] = children
		}
	}
	return nd
}

// ---------- Conditional branch (predicate-based) ----------

// ConditionalBranchCase is one arm of a predicate-based branch.
type ConditionalBranchCase struct {
	condition   *Condition
	conditionFn func(ctx *CallbackContext) bool
	nodes       []Node
	isDefault   bool
}

// When creates a conditional branch case with a declarative condition.
func When(cond *Condition, nodes ...Node) ConditionalBranchCase {
	return ConditionalBranchCase{condition: cond, nodes: nodes}
}

// WhenFunc creates a conditional branch case with a WASM callback condition.
func WhenFunc(fn func(ctx *CallbackContext) bool, nodes ...Node) ConditionalBranchCase {
	return ConditionalBranchCase{conditionFn: fn, nodes: nodes}
}

// Otherwise creates the fallback conditional branch case.
func Otherwise(nodes ...Node) ConditionalBranchCase {
	return ConditionalBranchCase{isDefault: true, nodes: nodes}
}

type conditionalBranchNode struct {
	cases []ConditionalBranchCase
}

// ConditionalBranch creates a predicate-based branch in the command flow.
func ConditionalBranch(cases ...ConditionalBranchCase) Node {
	return &conditionalBranchNode{cases: cases}
}

func (cb *conditionalBranchNode) toNodeDef(cmdName string, reg callbackMap) nodeDef {
	nd := nodeDef{Type: "conditional_branch"}
	for _, c := range cb.cases {
		children := make([]nodeDef, len(c.nodes))
		for i, n := range c.nodes {
			children[i] = n.toNodeDef(cmdName, reg)
		}
		if c.isDefault {
			nd.Default = children
			continue
		}
		cc := condCaseDef{Nodes: children}
		if c.condition != nil {
			cc.Condition = c.condition.toCondDef()
		}
		if c.conditionFn != nil {
			cbName := cmdName + ":cond:" + strconv.Itoa(len(reg))
			reg[cbName] = c.conditionFn
			cc.ConditionFn = cbName
		}
		nd.ConditionalCases = append(nd.ConditionalCases, cc)
	}
	return nd
}

// ---------- Declarative conditions ----------

// Condition is a declarative condition evaluated on the host side without a
// WASM callback. Use for step visibility ([StepBuilder.VisibleWhen]) or branch
// predicates ([When]).
type Condition struct {
	param    string
	op       string // "eq", "neq", "match", "set"
	value    string
	logicOp  string       // "and", "or", "not"
	children []*Condition // for "and", "or"
	child    *Condition   // for "not"
}

// ParamEq matches when the collected parameter equals value.
func ParamEq(param, value string) *Condition {
	return &Condition{param: param, op: "eq", value: value}
}

// ParamNeq matches when the collected parameter does not equal value.
func ParamNeq(param, value string) *Condition {
	return &Condition{param: param, op: "neq", value: value}
}

// ParamMatch matches when the collected parameter matches a regex pattern.
func ParamMatch(param, pattern string) *Condition {
	return &Condition{param: param, op: "match", value: pattern}
}

// ParamSet matches when the parameter has been collected (key exists in params).
func ParamSet(param string) *Condition {
	return &Condition{param: param, op: "set"}
}

// And requires all conditions to match.
func And(conditions ...*Condition) *Condition {
	return &Condition{logicOp: "and", children: conditions}
}

// Or requires at least one condition to match.
func Or(conditions ...*Condition) *Condition {
	return &Condition{logicOp: "or", children: conditions}
}

// Not negates a condition.
func Not(condition *Condition) *Condition {
	return &Condition{logicOp: "not", child: condition}
}

func (c *Condition) toCondDef() *conditionDef {
	if c == nil {
		return nil
	}
	cd := &conditionDef{}
	switch c.logicOp {
	case "and":
		cd.And = make([]*conditionDef, len(c.children))
		for i, ch := range c.children {
			cd.And[i] = ch.toCondDef()
		}
	case "or":
		cd.Or = make([]*conditionDef, len(c.children))
		for i, ch := range c.children {
			cd.Or[i] = ch.toCondDef()
		}
	case "not":
		cd.Not = c.child.toCondDef()
	default:
		cd.Param = c.param
		switch c.op {
		case "eq":
			v := c.value
			cd.Eq = &v
		case "neq":
			v := c.value
			cd.Neq = &v
		case "match":
			cd.Match = c.value
		case "set":
			t := true
			cd.Set = &t
		}
	}
	return cd
}
