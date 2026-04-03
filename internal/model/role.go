package model

type UserRole struct {
	ID       int64        `json:"id"`
	UserID   GlobalUserID `json:"user_id"`
	RoleType RoleLayer    `json:"role_type"`
	RoleName string       `json:"role_name"`
}
