package model

// NotifyPriority determines delivery behavior for notifications.
type NotifyPriority int

const (
	PriorityLow      NotifyPriority = iota // Informational, silent outside work hours
	PriorityNormal                         // Standard notification with sound
	PriorityHigh                           // Important — auto-mention user
	PriorityCritical                       // Urgent — mention, all channels, never silent
)

// NotificationPrefs stores per-user notification preferences.
type NotificationPrefs struct {
	GlobalUserID    GlobalUserID  `json:"global_user_id"`
	ChannelPriority []ChannelType `json:"channel_priority"`
	MuteMentions    bool          `json:"mute_mentions"`
	WorkHoursStart  *int          `json:"work_hours_start,omitempty"`
	WorkHoursEnd    *int          `json:"work_hours_end,omitempty"`
	Timezone        string        `json:"timezone"`
}

// SendOptions controls delivery-level behavior (silent mode, mention stripping).
type SendOptions struct {
	Silent        bool
	StripMentions bool
}

// StripMentionBlocks returns a copy of msg with all MentionBlock entries removed.
func StripMentionBlocks(msg Message) Message {
	filtered := make([]ContentBlock, 0, len(msg.Blocks))
	for _, block := range msg.Blocks {
		if _, isMention := block.(MentionBlock); !isMention {
			filtered = append(filtered, block)
		}
	}
	return Message{Blocks: filtered}
}
