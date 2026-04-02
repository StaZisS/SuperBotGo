package model

import "strings"

type UserInput interface {
	TextValue() string
	IsCommand() bool
	CommandName() string

	userInput()
}

// parseCommandName extracts the command name from a "/command[@bot] [args]" string.
func parseCommandName(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	name := strings.TrimPrefix(text, "/")
	if idx := strings.IndexAny(name, " @"); idx >= 0 {
		name = name[:idx]
	}
	return name
}

type TextInput struct {
	Text string
}

func (t TextInput) TextValue() string   { return t.Text }
func (t TextInput) IsCommand() bool     { return strings.HasPrefix(t.Text, "/") }
func (t TextInput) CommandName() string { return parseCommandName(t.Text) }
func (TextInput) userInput()            {}

type CallbackInput struct {
	Data  string
	Label string
}

func (c CallbackInput) TextValue() string   { return c.Data }
func (c CallbackInput) IsCommand() bool     { return strings.HasPrefix(c.Data, "/") }
func (c CallbackInput) CommandName() string { return parseCommandName(c.Data) }
func (CallbackInput) userInput()            {}
