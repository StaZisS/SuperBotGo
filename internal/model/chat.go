package model

type ChatKind string

const (
	ChatKindGroup   ChatKind = "GROUP"
	ChatKindPrivate ChatKind = "PRIVATE"
	ChatKindChannel ChatKind = "CHANNEL"
)
