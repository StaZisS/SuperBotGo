package notification

import (
	"context"
	"sync"
	"time"
)

// MemoryScheduledStore is a process-local fallback used by tests and non-Postgres wiring.
type MemoryScheduledStore struct {
	mu     sync.Mutex
	nextID int64
	rows   []memoryScheduledRow
}

type memoryScheduledRow struct {
	msg        ScheduledMessage
	lockedTill time.Time
	lastError  string
}

func NewMemoryScheduledStore() *MemoryScheduledStore {
	return &MemoryScheduledStore{}
}

func (s *MemoryScheduledStore) Enqueue(_ context.Context, msg ScheduledMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ID == 0 {
		s.nextID++
		msg.ID = s.nextID
	}
	s.rows = append(s.rows, memoryScheduledRow{msg: msg})
	return nil
}

func (s *MemoryScheduledStore) ClaimDue(_ context.Context, now time.Time, limit int, lease time.Duration) ([]ScheduledMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return nil, nil
	}

	claimed := make([]ScheduledMessage, 0, limit)
	lockedTill := now.Add(lease)
	for i := range s.rows {
		row := &s.rows[i]
		if len(claimed) >= limit {
			break
		}
		if row.msg.SendAt.After(now) || row.lockedTill.After(now) {
			continue
		}
		row.lockedTill = lockedTill
		row.msg.Attempts++
		claimed = append(claimed, row.msg)
	}
	return claimed, nil
}

func (s *MemoryScheduledStore) Complete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, row := range s.rows {
		if row.msg.ID == id {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (s *MemoryScheduledStore) Reschedule(_ context.Context, id int64, sendAt time.Time, reason error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.rows {
		if s.rows[i].msg.ID == id {
			s.rows[i].msg.SendAt = sendAt
			s.rows[i].lockedTill = time.Time{}
			if reason != nil {
				s.rows[i].lastError = reason.Error()
			}
			return nil
		}
	}
	return nil
}

func (s *MemoryScheduledStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

func (s *MemoryScheduledStore) Snapshot() []ScheduledMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ScheduledMessage, len(s.rows))
	for i, row := range s.rows {
		out[i] = row.msg
	}
	return out
}

var _ ScheduledMessageStore = (*MemoryScheduledStore)(nil)
