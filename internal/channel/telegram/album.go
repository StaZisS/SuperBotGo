package telegram

import (
	"sync"
	"time"

	"SuperBotGo/internal/model"
)

const albumFlushDelay = 500 * time.Millisecond

// albumEntry holds buffered data for one file in an album.
type albumEntry struct {
	ref      model.FileRef
	caption  string
	chatID   string
	userID   string
	updateID string
	username string
}

// albumBuffer collects files that arrive as a Telegram media group (same AlbumID)
// and flushes them as a single FileInput after a short delay.
type albumBuffer struct {
	mu      sync.Mutex
	albums  map[string]*pendingAlbum
	onFlush func(album *pendingAlbum)
}

type pendingAlbum struct {
	entries []albumEntry
	timer   *time.Timer
}

func newAlbumBuffer(onFlush func(*pendingAlbum)) *albumBuffer {
	return &albumBuffer{
		albums:  make(map[string]*pendingAlbum),
		onFlush: onFlush,
	}
}

// add buffers a file for the given albumID. If this is the first file in the
// album, a flush timer is started. Returns false if the file should be
// processed immediately (no albumID).
func (ab *albumBuffer) add(albumID string, entry albumEntry) bool {
	if albumID == "" {
		return false // no album — process immediately
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	pending, exists := ab.albums[albumID]
	if !exists {
		pending = &pendingAlbum{}
		ab.albums[albumID] = pending

		aid := albumID
		pending.timer = time.AfterFunc(albumFlushDelay, func() {
			ab.flush(aid)
		})
	} else {
		// Reset the timer so we wait 500ms after the *last* photo,
		// not 500ms after the first. This gives slower downloads
		// time to finish before the album is flushed.
		pending.timer.Reset(albumFlushDelay)
	}

	pending.entries = append(pending.entries, entry)
	return true
}

func (ab *albumBuffer) flush(albumID string) {
	ab.mu.Lock()
	pending, ok := ab.albums[albumID]
	if ok {
		delete(ab.albums, albumID)
	}
	ab.mu.Unlock()

	if ok && len(pending.entries) > 0 && ab.onFlush != nil {
		ab.onFlush(pending)
	}
}
