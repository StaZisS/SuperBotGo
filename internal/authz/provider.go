package authz

import "context"

type AttributeProvider interface {
	LoadAttributes(ctx context.Context, sc *SubjectContext) error
}

type SchemaContributor interface {
	ContributeConditions(ctx context.Context) []RuleConditionType
	ContributeFieldValues(ctx context.Context) map[string][]RuleParamOption
}
