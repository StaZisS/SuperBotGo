package state

import "SuperBotGo/internal/model"

// CommandNode is the sealed interface for all node types in the command tree.
// The commandNode method acts as a sealed marker: only types in this package
// can implement it.
type CommandNode interface {
	commandNode()
}

// OptionsPage holds a single page of options and whether more pages exist.
type OptionsPage struct {
	Options []model.Option
	HasMore bool
}

// PaginationConfig defines how paginated option selection works for a step.
type PaginationConfig struct {
	Prompt       string
	PageSize     int
	PageProvider func(ctx StepContext, page int) OptionsPage
}

// StepNode represents a single parameter-collection step in the command flow.
type StepNode struct {
	ParamName      string
	MessageBuilder func(ctx StepContext) model.Message
	Validate       func(model.UserInput) bool
	Condition      func(model.OptionMap) bool
	Pagination     *PaginationConfig
}

func (StepNode) commandNode() {}

// BranchNode branches the command flow based on the value of a previously
// collected parameter.
type BranchNode struct {
	OnParam string
	Cases   map[string][]CommandNode
	Default []CommandNode
}

func (BranchNode) commandNode() {}

// ConditionalCase pairs a predicate with a set of nodes.
type ConditionalCase struct {
	Predicate func(model.OptionMap) bool
	Nodes     []CommandNode
}

// ConditionalBranchNode branches the command flow based on predicate evaluation
// against the currently collected parameters.
type ConditionalBranchNode struct {
	Cases   []ConditionalCase
	Default []CommandNode
}

func (ConditionalBranchNode) commandNode() {}

// CommandDefinition describes a complete bot command: its name, description,
// access requirements, and the tree of steps/branches that collect parameters.
type CommandDefinition struct {
	Name         string
	Description  string
	Requirements *model.RoleRequirements
	Nodes        []CommandNode
}

// ResolveActiveSteps flattens the command node tree into the list of StepNodes
// that are currently active, given the already-collected params. It follows
// branches according to collected values and evaluates visibility conditions.
func (cd *CommandDefinition) ResolveActiveSteps(params model.OptionMap) []StepNode {
	return flattenNodes(cd.Nodes, params)
}

// CurrentStep returns the next step that needs user input, or nil if the
// command is complete.
func (cd *CommandDefinition) CurrentStep(params model.OptionMap) *StepNode {
	steps := cd.ResolveActiveSteps(params)
	for i := range steps {
		if _, exists := params[steps[i].ParamName]; !exists {
			return &steps[i]
		}
	}
	return nil
}

// IsComplete returns true if all active steps have been filled in.
func (cd *CommandDefinition) IsComplete(params model.OptionMap) bool {
	return cd.CurrentStep(params) == nil
}

// flattenNodes recursively resolves the node tree into a flat list of active
// StepNodes based on the current parameter values.
func flattenNodes(nodes []CommandNode, params model.OptionMap) []StepNode {
	var result []StepNode
	for _, node := range nodes {
		switch n := node.(type) {
		case StepNode:

			if n.Condition == nil || n.Condition(params) {
				result = append(result, n)
			}
		case BranchNode:
			value, exists := params[n.OnParam]
			var branch []CommandNode
			if exists {
				if caseNodes, ok := n.Cases[value]; ok {
					branch = caseNodes
				} else {
					branch = n.Default
				}
			} else {
				branch = n.Default
			}
			if branch != nil {
				result = append(result, flattenNodes(branch, params)...)
			}
		case ConditionalBranchNode:
			var matched []CommandNode
			for _, c := range n.Cases {
				if c.Predicate(params) {
					matched = c.Nodes
					break
				}
			}
			if matched == nil {
				matched = n.Default
			}
			if matched != nil {
				result = append(result, flattenNodes(matched, params)...)
			}
		}
	}
	return result
}
