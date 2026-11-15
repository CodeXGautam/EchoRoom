// Package room runs the live state of a room: who is in it and where their
// doodles are.
//
// Concurrency model ("actor" pattern): each Room has exactly one goroutine,
// Run, that owns all mutable state (clients, players). Everything else talks
// to it through channels. No mutexes, no data races by construction, and the
// order of events inside a room is simply the order the loop receives them.
package room

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"
)

const (
	// TickInterval is how often batched movement is broadcast (10 Hz).
	TickInterval = 100 * time.Millisecond

	// MaxSpeed is the fastest a doodle may walk, in world pixels per second.
	// The frontend uses the same number.
	MaxSpeed = 220.0

	// moveSlack forgives network jitter: two packets arriving together look
	// like a burst of speed. Extra allowance = MaxSpeed × moveSlack.
	moveSlack = 150 * time.Millisecond

	// minMoveInterval drops moves arriving faster than ~50/s.
	minMoveInterval = 20 * time.Millisecond
)

var DefaultWorld = World{W: 1280, H: 800}

// Player is the public, broadcastable view of someone in the room.
type Player struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Avatar Avatar  `json:"avatar"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Dir    string  `json:"dir"`
	Moving bool    `json:"moving"`

	lastMove time.Time
}

// PlayerState is the small per-tick movement update.
type PlayerState struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Dir    string  `json:"dir"`
	Moving bool    `json:"moving"`
}

type inbound struct {
	client *Client
	move   moveMsg
}

type Room struct {
	ID    string
	world World
	log   *slog.Logger
	now   func() time.Time

	join    chan *Client
	leave   chan *Client
	inbound chan inbound
	done    chan struct{}

	// Owned by Run. Never touch from other goroutines.
	clients map[string]*Client // userID → current connection
	players map[string]*Player // userID → doodle
	dirty   map[string]struct{}
}

func New(id string, log *slog.Logger) *Room {
	return &Room{
		ID:      id,
		world:   DefaultWorld,
		log:     log.With("room", id),
		now:     time.Now,
		join:    make(chan *Client),
		leave:   make(chan *Client),
		inbound: make(chan inbound, 256),
		done:    make(chan struct{}),
		clients: map[string]*Client{},
		players: map[string]*Player{},
		dirty:   map[string]struct{}{},
	}
}

// --- API used by the transport (any goroutine) ---

// Join adds c to the room. It fails only if the room has stopped or ctx ends.
func (r *Room) Join(ctx context.Context, c *Client) error {
	select {
	case r.join <- c:
		return nil
	case <-r.done:
		return context.Canceled
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Leave removes c. Never blocks after the room has stopped.
func (r *Room) Leave(c *Client) {
	select {
	case r.leave <- c:
	case <-r.done:
	}
}

// Handle decodes one frame from c. Decoding happens here, on the reader's
// goroutine, so the room loop only does cheap work.
func (r *Room) Handle(ctx context.Context, c *Client, frame []byte) {
	var env envelope
	if err := json.Unmarshal(frame, &env); err != nil {
		return
	}
	switch env.T {
	case "move":
		var m moveMsg
		if err := json.Unmarshal(env.D, &m); err != nil {
			return
		}
		select {
		case r.inbound <- inbound{client: c, move: m}:
		case <-r.done:
		case <-ctx.Done():
		}
	default:
		// Unknown types are ignored so newer clients don't break older servers.
	}
}

// --- The loop ---

func (r *Room) Run(ctx context.Context) {
	ticker := time.NewTicker(TickInterval)
	defer ticker.Stop()
	defer close(r.done)

	r.log.Info("room_started")
	for {
		select {
		case <-ctx.Done():
			for _, c := range r.clients {
				c.Kick(CloseShutdown, "server shutting down")
			}
			r.log.Info("room_stopped")
			return
		case c := <-r.join:
			r.addClient(c)
		case c := <-r.leave:
			r.removeClient(c)
		case in := <-r.inbound:
			r.applyMove(in)
		case <-ticker.C:
			r.flush()
		}
	}
}

func (r *Room) addClient(c *Client) {
	p, exists := r.players[c.UserID]
	if old := r.clients[c.UserID]; old != nil {
		// Same user in a new tab: the new tab takes over the existing doodle.
		old.Kick(CloseReplaced, "opened in another tab")
	}
	r.clients[c.UserID] = c

	if !exists {
		p = &Player{
			ID:       c.UserID,
			Name:     c.Profile.Name,
			Avatar:   c.Profile.Avatar,
			X:        round1(float64(r.world.W)/2 + (rand.Float64()-0.5)*200),
			Y:        round1(float64(r.world.H)/2 + (rand.Float64()-0.5)*120),
			Dir:      "right",
			lastMove: r.now(),
		}
		r.players[c.UserID] = p
		r.broadcast(encode("joined", p), c.UserID)
	}

	players := make([]Player, 0, len(r.players))
	for _, pl := range r.players {
		players = append(players, *pl)
	}
	c.trySend(encode("welcome", welcomeMsg{SelfID: c.UserID, World: r.world, Players: players}))
	r.log.Info("player_joined", "user", c.UserID, "online", len(r.clients))
}

func (r *Room) removeClient(c *Client) {
	// A replaced tab's late Leave must not remove the new tab's doodle.
	if r.clients[c.UserID] != c {
		return
	}
	delete(r.clients, c.UserID)
	delete(r.players, c.UserID)
	delete(r.dirty, c.UserID)
	r.broadcast(encode("left", leftMsg{ID: c.UserID}), "")
	r.log.Info("player_left", "user", c.UserID, "online", len(r.clients))
}

// applyMove trusts the client's position (so movement feels instant) but
// rejects anything faster than MaxSpeed allows. This is "client-authoritative
// with server validation"; it stops teleport cheats, not subtle speed hacks.
func (r *Room) applyMove(in inbound) {
	if r.clients[in.client.UserID] != in.client {
		return // stale connection
	}
	p := r.players[in.client.UserID]
	m := in.move
	if !finite(m.X) || !finite(m.Y) {
		return
	}

	now := r.now()
	dt := now.Sub(p.lastMove)
	if dt < minMoveInterval {
		return
	}
	dt = min(dt, time.Second) // idling doesn't bank a teleport

	x := clamp(m.X, 0, float64(r.world.W))
	y := clamp(m.Y, 0, float64(r.world.H))
	allowed := MaxSpeed * (dt + moveSlack).Seconds()
	if math.Hypot(x-p.X, y-p.Y) > allowed {
		in.client.trySend(encode("correct", correctMsg{X: p.X, Y: p.Y}))
		p.lastMove = now
		return
	}

	p.X, p.Y = round1(x), round1(y)
	p.Moving = m.Moving
	if m.Dir == "left" || m.Dir == "right" {
		p.Dir = m.Dir
	}
	p.lastMove = now
	r.dirty[p.ID] = struct{}{}
}

// flush sends one batched update per tick instead of one frame per move.
// With N players moving, that's N frames/tick total instead of N² frames.
func (r *Room) flush() {
	if len(r.dirty) == 0 {
		return
	}
	states := make([]PlayerState, 0, len(r.dirty))
	for id := range r.dirty {
		p := r.players[id]
		states = append(states, PlayerState{ID: p.ID, X: p.X, Y: p.Y, Dir: p.Dir, Moving: p.Moving})
	}
	clear(r.dirty)
	r.broadcast(encode("state", stateMsg{Players: states}), "")
}

func (r *Room) broadcast(frame []byte, except string) {
	for id, c := range r.clients {
		if id != except {
			c.trySend(frame)
		}
	}
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }
