package model

import "strings"

type UserInput interface {
	TextValue() string
	IsCommand() bool
	CommandName() string

	userInput()
}

type TextInput struct {
	Text string
}

func (t TextInput) TextValue() string { return t.Text }
func (t TextInput) IsCommand() bool   { return strings.HasPrefix(t.Text, "/") }
func (t TextInput) CommandName() string {
	if !t.IsCommand() {
		return ""
	}

	name := strings.TrimPrefix(t.Text, "/")
	if idx := strings.IndexAny(name, " @"); idx >= 0 {
		name = name[:idx]
	}
	return name
}
func (TextInput) userInput() {}

type CallbackInput struct {
	Data  string
	Label string
}

func (c CallbackInput) TextValue() string { return c.Data }
func (c CallbackInput) IsCommand() bool   { return strings.HasPrefix(c.Data, "/") }
func (c CallbackInput) CommandName() string {
	if !c.IsCommand() {
		return ""
	}
	name := strings.TrimPrefix(c.Data, "/")
	if idx := strings.IndexAny(name, " @"); idx >= 0 {
		name = name[:idx]
	}
	return name
}
func (CallbackInput) userInput() {}
