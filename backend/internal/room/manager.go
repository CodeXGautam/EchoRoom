package room

import (
	"context"
	"log/slog"
	"sync"
)

// Manager owns every Room running in this process and starts them lazily.
//
// Phase 1 only uses "lobby". Rooms are never stopped while the process runs;
// Phase 2 (quick rooms) adds stopping a room once it has been empty for a while.
type Manager struct {
	ctx context.Context
	log *slog.Logger

	mu    sync.Mutex
	rooms map[string]*Room
	wg    sync.WaitGroup
}

// NewManager's rooms stop when ctx is cancelled; call Wait afterwards.
func NewManager(ctx context.Context, log *slog.Logger) *Manager {
	return &Manager{ctx: ctx, log: log, rooms: map[string]*Room{}}
}

func (m *Manager) Get(id string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rooms[id]; ok {
		return r
	}
	r := New(id, m.log)
	m.rooms[id] = r
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		r.Run(m.ctx)
	}()
	return r
}

// Wait blocks until every room loop has exited (after ctx is cancelled).
func (m *Manager) Wait() { m.wg.Wait() }
