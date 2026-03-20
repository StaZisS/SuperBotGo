package state

import "SuperBotGo/internal/model"

type CommandNode interface {
	commandNode()
}

type OptionsPage struct {
	Options []model.Option
	HasMore bool
}

type PaginationConfig struct {
	Prompt       string
	PageSize     int
	PageProvider func(ctx StepContext, page int) OptionsPage
}

type StepNode struct {
	ParamName      string
	MessageBuilder func(ctx StepContext) model.Message
	Validate       func(model.UserInput) bool
	Condition      func(model.OptionMap) bool
	Pagination     *PaginationConfig
}

func (StepNode) commandNode() {}

type BranchNode struct {
	OnParam string
	Cases   map[string][]CommandNode
	Default []CommandNode
}

func (BranchNode) commandNode() {}

type ConditionalCase struct {
	Predicate func(model.OptionMap) bool
	Nodes     []CommandNode
}

type ConditionalBranchNode struct {
	Cases   []ConditionalCase
	Default []CommandNode
}

func (ConditionalBranchNode) commandNode() {}

type CommandDefinition struct {
	Name         string
	Description  string
	Requirements *model.RoleRequirements
	Nodes        []CommandNode
}

func (cd *CommandDefinition) ResolveActiveSteps(params model.OptionMap) []StepNode {
	return flattenNodes(cd.Nodes, params)
}

func (cd *CommandDefinition) CurrentStep(params model.OptionMap) *StepNode {
	steps := cd.ResolveActiveSteps(params)
	for i := range steps {
		if _, exists := params[steps[i].ParamName]; !exists {
			return &steps[i]
		}
	}
	return nil
}

func (cd *CommandDefinition) IsComplete(params model.OptionMap) bool {
	return cd.CurrentStep(params) == nil
}

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
