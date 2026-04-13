package discord

import "testing"

func TestChannelTrackerReplaceAndRemoveGuild(t *testing.T) {
	tracker := newChannelTracker()

	tracker.ReplaceGuild("guild-1", []string{"ch-1", "ch-2"})
	if !tracker.Has("ch-1") || !tracker.Has("ch-2") {
		t.Fatal("ReplaceGuild did not track channels")
	}

	removed := tracker.RemoveGuild("guild-1")
	if len(removed) != 2 {
		t.Fatalf("RemoveGuild() removed %d channels, want 2", len(removed))
	}
	if tracker.Has("ch-1") || tracker.Has("ch-2") {
		t.Fatal("RemoveGuild() did not clear tracked channels")
	}
}

func TestChannelTrackerUpsertAndRemove(t *testing.T) {
	tracker := newChannelTracker()

	tracker.Upsert("guild-1", "ch-1")
	if !tracker.Has("ch-1") {
		t.Fatal("Upsert() did not track the channel")
	}

	guildID, ok := tracker.Remove("ch-1")
	if !ok || guildID != "guild-1" {
		t.Fatalf("Remove() = (%q, %v), want (%q, true)", guildID, ok, "guild-1")
	}
	if tracker.Has("ch-1") {
		t.Fatal("Remove() did not untrack the channel")
	}
}
