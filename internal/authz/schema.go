package authz

import "context"

type RuleParamOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type RuleParam struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Type        string            `json:"type"`
	Placeholder string            `json:"placeholder,omitempty"`
	Options     []RuleParamOption `json:"options,omitempty"`
	DependsOn   string            `json:"depends_on,omitempty"`
}

type RuleConditionType struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Template string      `json:"template"`
	Params   []RuleParam `json:"params"`
}

type RuleSchema struct {
	ConditionTypes []RuleConditionType          `json:"condition_types"`
	FieldValues    map[string][]RuleParamOption `json:"field_values"`
}

type RuleSchemaBuilder struct {
	store        Store
	contributors []SchemaContributor
}

func NewRuleSchemaBuilder(store Store, contributors ...SchemaContributor) *RuleSchemaBuilder {
	return &RuleSchemaBuilder{store: store, contributors: contributors}
}

func (b *RuleSchemaBuilder) Build(ctx context.Context) RuleSchema {
	var conditions []RuleConditionType
	fieldValues := make(map[string][]RuleParamOption)

	for _, c := range b.contributors {
		conditions = append(conditions, c.ContributeConditions(ctx)...)
		for k, v := range c.ContributeFieldValues(ctx) {
			fieldValues[k] = v
		}
	}

	conditions = append(conditions, b.buildRoleType(ctx))

	return RuleSchema{
		ConditionTypes: conditions,
		FieldValues:    fieldValues,
	}
}

func (b *RuleSchemaBuilder) buildRoleType(ctx context.Context) RuleConditionType {
	roleOptions := b.store.GetDistinctRoleNames(ctx)

	paramType := "text"
	if len(roleOptions) > 0 {
		paramType = "text_or_select"
	}

	var opts []RuleParamOption
	for _, name := range roleOptions {
		opts = append(opts, RuleParamOption{Value: name, Label: name})
	}

	return RuleConditionType{
		ID:       "role",
		Label:    "Имеет роль",
		Template: `has_role("{roleName}")`,
		Params: []RuleParam{
			{
				Name:        "roleName",
				Label:       "Роль",
				Type:        paramType,
				Placeholder: "имя роли (напр. ADMIN)",
				Options:     opts,
			},
		},
	}
}
