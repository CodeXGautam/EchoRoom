package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeXGautam/echoroom/internal/auth"
	"github.com/CodeXGautam/echoroom/internal/middleware"
	"github.com/CodeXGautam/echoroom/internal/room"
	"github.com/alicebob/miniredis/v2"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func newStore(t *testing.T) *TicketStore {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewTicketStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
}

func TestTicketIsSingleUse(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id, err := s.Issue(ctx, Claim{UserID: "u1", RoomID: LobbyID})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Redeem(ctx, id)
	if err != nil || c.UserID != "u1" {
		t.Fatalf("first redeem: %+v %v", c, err)
	}
	if _, err := s.Redeem(ctx, id); err != ErrTicketInvalid {
		t.Fatalf("second redeem err = %v, want ErrTicketInvalid", err)
	}
	if _, err := s.Redeem(ctx, "nope"); err != ErrTicketInvalid {
		t.Fatalf("unknown ticket err = %v", err)
	}
}

// TestEndToEnd drives the real router + middleware stack: get a ticket over
// HTTP, open two sockets, move one doodle, see it on the other socket.
func TestEndToEnd(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	rooms := room.NewManager(ctx, log)
	t.Cleanup(func() { cancel(); rooms.Wait() })

	r := chi.NewRouter()
	r.Use(middleware.Logging(log)) // regression: wrapper must not break Hijack
	r.With(auth.Middleware(auth.DevVerifier{})).Post("/api/v1/ws/ticket", TicketHandler(store, log))
	// Same-origin test server, so no OriginPatterns needed.
	r.Method(http.MethodGet, "/ws", &Handler{Tickets: store, Rooms: rooms, Log: log})

	srv := httptest.NewUnstartedServer(r)
	// Reproduce production timeouts: the socket must outlive ReadTimeout.
	srv.Config.ReadTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)

	connect := func(devID, name string) *websocket.Conn {
		body := `{"profile":{"name":"` + name + `","avatar":{"color":"#aabbcc"}}}`
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/ws/ticket", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer dev:"+devID)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("ticket: status=%v err=%v", res.StatusCode, err)
		}
		var tr ticketResponse
		_ = json.NewDecoder(res.Body).Decode(&tr)
		res.Body.Close()

		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?ticket="+tr.Ticket, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { c.CloseNow() })
		return c
	}

	read := func(c *websocket.Conn, typ string) json.RawMessage {
		rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
		defer rcancel()
		for {
			_, data, err := c.Read(rctx)
			if err != nil {
				t.Fatalf("read %s: %v", typ, err)
			}
			var env envelopeT
			_ = json.Unmarshal(data, &env)
			if env.T == typ {
				return env.D
			}
		}
	}

	alice := connect("alice-0001", "Alice")
	var w struct {
		Players []room.Player `json:"players"`
	}
	_ = json.Unmarshal(read(alice, "welcome"), &w)
	start := w.Players[0]

	bob := connect("bob-00001", "Bob")
	read(bob, "welcome")
	read(alice, "joined")

	// Wait past the server ReadTimeout to prove deadlines were cleared.
	time.Sleep(500 * time.Millisecond)

	move, _ := json.Marshal(map[string]any{"t": "move", "d": map[string]any{
		"x": start.X + 20, "y": start.Y, "dir": "right", "moving": true,
	}})
	if err := alice.Write(ctx, websocket.MessageText, move); err != nil {
		t.Fatalf("write after ReadTimeout: %v", err)
	}

	var s struct {
		Players []room.PlayerState `json:"players"`
	}
	_ = json.Unmarshal(read(bob, "state"), &s)
	if len(s.Players) != 1 || s.Players[0].ID != "dev-alice-0001" {
		t.Fatalf("bob got state %+v", s)
	}

	// The ticket was consumed: reusing a bad ticket must be refused.
	_, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?ticket=reused", nil)
	if err == nil || res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad ticket: err=%v res=%v", err, res)
	}
}

type envelopeT struct {
	T string          `json:"t"`
	D json.RawMessage `json:"d"`
}
