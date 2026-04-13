package discord

import "sync"

type channelTracker struct {
	mu            sync.RWMutex
	guildChannels map[string]map[string]struct{}
	channelGuild  map[string]string
}

func newChannelTracker() *channelTracker {
	return &channelTracker{
		guildChannels: make(map[string]map[string]struct{}),
		channelGuild:  make(map[string]string),
	}
}

func (t *channelTracker) Upsert(guildID, channelID string) {
	if guildID == "" || channelID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	channels := t.guildChannels[guildID]
	if channels == nil {
		channels = make(map[string]struct{})
		t.guildChannels[guildID] = channels
	}
	channels[channelID] = struct{}{}
	t.channelGuild[channelID] = guildID
}

func (t *channelTracker) ReplaceGuild(guildID string, channelIDs []string) {
	if guildID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if existing := t.guildChannels[guildID]; existing != nil {
		for channelID := range existing {
			delete(t.channelGuild, channelID)
		}
	}

	channels := make(map[string]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID == "" {
			continue
		}
		channels[channelID] = struct{}{}
		t.channelGuild[channelID] = guildID
	}
	t.guildChannels[guildID] = channels
}

func (t *channelTracker) Remove(channelID string) (string, bool) {
	if channelID == "" {
		return "", false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	guildID, ok := t.channelGuild[channelID]
	if !ok {
		return "", false
	}

	delete(t.channelGuild, channelID)
	if channels := t.guildChannels[guildID]; channels != nil {
		delete(channels, channelID)
		if len(channels) == 0 {
			delete(t.guildChannels, guildID)
		}
	}
	return guildID, true
}

func (t *channelTracker) RemoveGuild(guildID string) []string {
	if guildID == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	channels := t.guildChannels[guildID]
	if len(channels) == 0 {
		delete(t.guildChannels, guildID)
		return nil
	}

	channelIDs := make([]string, 0, len(channels))
	for channelID := range channels {
		channelIDs = append(channelIDs, channelID)
		delete(t.channelGuild, channelID)
	}
	delete(t.guildChannels, guildID)
	return channelIDs
}

func (t *channelTracker) Has(channelID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, ok := t.channelGuild[channelID]
	return ok
}
