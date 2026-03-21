package authz

import "SuperBotGo/internal/model"

// SubjectContext holds all authorization-relevant data about a user.
// Core fields (UserID, ExternalID, Roles, Groups) are always populated.
// Attrs is a dynamic map populated by registered AttributeProvider instances.
type SubjectContext struct {
	UserID         model.GlobalUserID
	ExternalID     string
	Roles          []string
	Groups         []string
	PrimaryChannel string
	Locale         string
	Attrs          map[string]any
}
