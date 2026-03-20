package model

import "strings"

// UserInput is a sealed interface representing input received from a user.
// Only types in this package may implement it.
type UserInput interface {
	// TextValue returns the textual representation of the input.
	TextValue() string
	// IsCommand reports whether the input is a bot command (starts with /).
	IsCommand() bool
	// CommandName returns the command name without the leading slash,
	// or an empty string if the input is not a command.
	CommandName() string

	// userInput is an unexported method that seals the interface.
	userInput()
}

// TextInput represents plain text or a command typed by the user.
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

// CallbackInput represents a callback triggered by an inline button press.
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
