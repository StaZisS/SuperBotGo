package resume

import "SuperBotGo/internal/state"

func ResumeCommand() *state.CommandDefinition {
	return state.NewCommand("resume").
		Description("Resume active command on this platform").
		PreservesDialog().
		Build()
}
