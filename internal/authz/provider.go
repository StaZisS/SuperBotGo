package authz

import "context"

// AttributeProvider loads domain-specific attributes into SubjectContext.
// Each domain (university students, HR employees, etc.) registers its own provider.
type AttributeProvider interface {
	// LoadAttributes populates sc.Attrs with domain-specific key-value pairs.
	// Keys become accessible in policy expressions via user.{key}.
	LoadAttributes(ctx context.Context, sc *SubjectContext) error
}

// SchemaContributor provides UI schema fragments for the admin rule editor.
// An AttributeProvider may optionally implement this interface.
type SchemaContributor interface {
	ContributeConditions(ctx context.Context) []RuleConditionType
	ContributeFieldValues(ctx context.Context) map[string][]RuleParamOption
}
