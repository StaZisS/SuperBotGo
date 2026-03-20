package api

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

type RuleSchemaHandler struct {
	pool *pgxpool.Pool
}

func NewRuleSchemaHandler(pool *pgxpool.Pool) *RuleSchemaHandler {
	return &RuleSchemaHandler{pool: pool}
}

func (h *RuleSchemaHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/rule-schema", h.handleGetSchema)
}

func (h *RuleSchemaHandler) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	schema := h.buildSchema(r.Context())
	writeJSON(w, http.StatusOK, schema)
}

func (h *RuleSchemaHandler) buildSchema(ctx context.Context) RuleSchema {
	return RuleSchema{
		ConditionTypes: []RuleConditionType{
			h.buildAttributeType(ctx),
			h.buildGraphType(ctx),
			h.buildRoleType(ctx),
		},
		FieldValues: h.buildFieldValues(ctx),
	}
}

func (h *RuleSchemaHandler) buildFieldValues(ctx context.Context) map[string][]RuleParamOption {
	fv := map[string][]RuleParamOption{
		"nationality_type": {{Value: "domestic", Label: "domestic"}, {Value: "foreign", Label: "foreign"}},
		"funding_type":     {{Value: "budget", Label: "budget"}, {Value: "contract", Label: "contract"}},
		"education_form":   {{Value: "full_time", Label: "full_time"}, {Value: "part_time", Label: "part_time"}, {Value: "remote", Label: "remote"}},
		"primary_channel":  {{Value: "TELEGRAM", Label: "TELEGRAM"}, {Value: "DISCORD", Label: "DISCORD"}},
	}

	if h.pool != nil {
		if vals := h.loadDistinctValues(ctx, "student_positions", "nationality_type"); len(vals) > 0 {
			fv["nationality_type"] = vals
		}
		if vals := h.loadDistinctValues(ctx, "student_positions", "funding_type"); len(vals) > 0 {
			fv["funding_type"] = vals
		}
		if vals := h.loadDistinctValues(ctx, "student_positions", "education_form"); len(vals) > 0 {
			fv["education_form"] = vals
		}
		if vals := h.loadDistinctValues(ctx, "channel_accounts", "channel_type"); len(vals) > 0 {
			fv["primary_channel"] = vals
		}
	}

	return fv
}

func (h *RuleSchemaHandler) buildAttributeType(ctx context.Context) RuleConditionType {
	fields := []RuleParamOption{
		{Value: "nationality_type", Label: "Гражданство"},
		{Value: "funding_type", Label: "Финансирование"},
		{Value: "education_form", Label: "Форма обучения"},
		{Value: "primary_channel", Label: "Канал"},
		{Value: "locale", Label: "Локаль"},
		{Value: "external_id", Label: "Внешний ID"},
	}

	return RuleConditionType{
		ID:       "attribute",
		Label:    "Атрибут пользователя",
		Template: `user.{field} {operator} "{value}"`,
		Params: []RuleParam{
			{
				Name:    "field",
				Label:   "Поле",
				Type:    "select",
				Options: fields,
			},
			{
				Name:  "operator",
				Label: "Оператор",
				Type:  "select",
				Options: []RuleParamOption{
					{Value: "==", Label: "="},
					{Value: "!=", Label: "!="},
				},
			},
			{
				Name:        "value",
				Label:       "Значение",
				Type:        "text_or_select",
				Placeholder: "значение...",
				DependsOn:   "field",
			},
		},
	}
}

func (h *RuleSchemaHandler) buildGraphType(ctx context.Context) RuleConditionType {
	relations := []RuleParamOption{
		{Value: "member", Label: "member (член)"},
		{Value: "teacher", Label: "teacher (преподаватель)"},
		{Value: "foreign_teacher", Label: "foreign_teacher (преп. иностранцев)"},
		{Value: "dean", Label: "dean (декан)"},
		{Value: "head", Label: "head (завкаф)"},
		{Value: "director", Label: "director (руководитель)"},
		{Value: "curator", Label: "curator (куратор)"},
	}

	if h.pool != nil {
		if vals := h.loadDistinctRelations(ctx); len(vals) > 0 {
			relations = vals
		}
	}

	objectTypes := []RuleParamOption{
		{Value: "faculty", Label: "Факультет"},
		{Value: "department", Label: "Кафедра"},
		{Value: "program", Label: "Направление"},
		{Value: "stream", Label: "Поток"},
		{Value: "group", Label: "Группа"},
		{Value: "subgroup", Label: "Подгруппа"},
	}

	if h.pool != nil {
		if vals := h.loadDistinctObjectTypes(ctx); len(vals) > 0 {
			objectTypes = vals
		}
	}

	return RuleConditionType{
		ID:       "graph",
		Label:    "Проверка по графу",
		Template: `check("{relation}", "{objectType}", "{objectId}")`,
		Params: []RuleParam{
			{
				Name:    "relation",
				Label:   "Связь",
				Type:    "select",
				Options: relations,
			},
			{
				Name:    "objectType",
				Label:   "Тип объекта",
				Type:    "select",
				Options: objectTypes,
			},
			{
				Name:        "objectId",
				Label:       "Код объекта",
				Type:        "text",
				Placeholder: "код...",
				DependsOn:   "objectType",
			},
		},
	}
}

func (h *RuleSchemaHandler) buildRoleType(_ context.Context) RuleConditionType {
	var roleOptions []RuleParamOption

	if h.pool != nil {
		rows, err := h.pool.Query(context.Background(), `
			SELECT DISTINCT role_name FROM user_roles ORDER BY role_name
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if rows.Scan(&name) == nil {
					roleOptions = append(roleOptions, RuleParamOption{Value: name, Label: name})
				}
			}
		}
	}

	paramType := "text"
	if len(roleOptions) > 0 {
		paramType = "text_or_select"
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
				Options:     roleOptions,
			},
		},
	}
}

func (h *RuleSchemaHandler) loadDistinctValues(ctx context.Context, table, column string) []RuleParamOption {
	allowedColumns := map[string]bool{
		"student_positions:nationality_type": true,
		"student_positions:funding_type":     true,
		"student_positions:education_form":   true,
		"channel_accounts:channel_type":      true,
	}
	key := table + ":" + column
	if !allowedColumns[key] {
		return nil
	}

	rows, err := h.pool.Query(ctx,
		"SELECT DISTINCT "+column+" FROM "+table+" WHERE "+column+" IS NOT NULL ORDER BY "+column)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var opts []RuleParamOption
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			opts = append(opts, RuleParamOption{Value: val, Label: val})
		}
	}
	return opts
}

func (h *RuleSchemaHandler) loadDistinctRelations(ctx context.Context) []RuleParamOption {
	rows, err := h.pool.Query(ctx, `
		SELECT DISTINCT relation FROM authorization_tuples
		WHERE relation NOT IN ('parent')
		ORDER BY relation
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	labels := map[string]string{
		"member":          "member (член)",
		"teacher":         "teacher (преподаватель)",
		"foreign_teacher": "foreign_teacher (преп. иностранцев)",
		"dean":            "dean (декан)",
		"head":            "head (завкаф)",
		"director":        "director (руководитель)",
		"curator":         "curator (куратор)",
		"executor":        "executor (исполнитель)",
	}

	var opts []RuleParamOption
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			lbl := val
			if l, ok := labels[val]; ok {
				lbl = l
			}
			opts = append(opts, RuleParamOption{Value: val, Label: lbl})
		}
	}
	return opts
}

func (h *RuleSchemaHandler) loadDistinctObjectTypes(ctx context.Context) []RuleParamOption {
	rows, err := h.pool.Query(ctx, `
		SELECT DISTINCT object_type FROM authorization_tuples
		WHERE object_type NOT IN ('plugin_command')
		ORDER BY object_type
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	labels := map[string]string{
		"faculty":    "Факультет",
		"department": "Кафедра",
		"program":    "Направление",
		"stream":     "Поток",
		"group":      "Группа",
		"subgroup":   "Подгруппа",
	}

	var opts []RuleParamOption
	for rows.Next() {
		var val string
		if rows.Scan(&val) == nil {
			lbl := val
			if l, ok := labels[val]; ok {
				lbl = l
			}
			opts = append(opts, RuleParamOption{Value: val, Label: lbl})
		}
	}
	return opts
}
