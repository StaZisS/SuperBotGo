package authz

import "SuperBotGo/internal/model"

type SubjectContext struct {
	UserID         model.GlobalUserID
	ExternalID     string
	Roles          []string
	Groups         []string
	PrimaryChannel string
	Locale         string
	Attrs          map[string]any
}
