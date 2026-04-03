package model

type RoleLayer string

const (
	RoleLayerSystem RoleLayer = "SYSTEM"
	RoleLayerGlobal RoleLayer = "GLOBAL"
	RoleLayerPlugin RoleLayer = "PLUGIN"
)
