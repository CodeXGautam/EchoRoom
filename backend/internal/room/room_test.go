package room

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// fakeClock is read by the room goroutine and advanced by the test.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func testRoom(t *testing.T) (*Room, *fakeClock) {
	t.Helper()
	r := New("test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	r.now = clock.Now
	ctx, cancel := context.WithCancel(context.Background())
	go r.Run(ctx)
	t.Cleanup(func() { cancel(); <-r.done })
	return r, clock
}

// next reads the next frame of type typ, skipping others, with a timeout.
func next(t *testing.T, c *Client, typ string) json.RawMessage {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case f := <-c.Send():
			var env envelope
			if err := json.Unmarshal(f, &env); err != nil {
				t.Fatalf("bad frame %s: %v", f, err)
			}
			if env.T == typ {
				return env.D
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q", typ)
		}
	}
}

func client(id string) *Client {
	return NewClient(id, Profile{Name: id, Avatar: Avatar{Color: "#ffb3c6"}})
}

func moveFrame(x, y float64) []byte {
	return encode("move", moveMsg{X: x, Y: y, Dir: "left", Moving: true})
}

func TestJoinSeesExistingPlayersAndOthersSeeJoin(t *testing.T) {
	r, _ := testRoom(t)
	ctx := context.Background()

	a := client("alice")
	if err := r.Join(ctx, a); err != nil {
		t.Fatal(err)
	}
	next(t, a, "welcome")

	b := client("bob")
	_ = r.Join(ctx, b)

	var w welcomeMsg
	_ = json.Unmarshal(next(t, b, "welcome"), &w)
	if w.SelfID != "bob" || len(w.Players) != 2 {
		t.Fatalf("bob welcome = %+v, want selfId bob and 2 players", w)
	}

	var joined Player
	_ = json.Unmarshal(next(t, a, "joined"), &joined)
	if joined.ID != "bob" {
		t.Fatalf("alice saw joined %q, want bob", joined.ID)
	}
}

func TestValidMoveIsBroadcastOnTick(t *testing.T) {
	r, clock := testRoom(t)
	ctx := context.Background()
	a, b := client("alice"), client("bob")
	_ = r.Join(ctx, a)
	_ = r.Join(ctx, b)
	var w welcomeMsg
	_ = json.Unmarshal(next(t, a, "welcome"), &w)
	var start Player
	for _, p := range w.Players {
		if p.ID == "alice" {
			start = p
		}
	}

	clock.Advance(200 * time.Millisecond) // 200ms × 220px/s ≈ 44px allowed (+slack)
	r.Handle(ctx, a, moveFrame(start.X+30, start.Y))

	var s stateMsg
	_ = json.Unmarshal(next(t, b, "state"), &s)
	if len(s.Players) != 1 || s.Players[0].ID != "alice" || s.Players[0].X != round1(start.X+30) {
		t.Fatalf("state = %+v", s)
	}
}

func TestTeleportIsRejectedWithCorrection(t *testing.T) {
	r, clock := testRoom(t)
	ctx := context.Background()
	a := client("alice")
	_ = r.Join(ctx, a)
	var w welcomeMsg
	_ = json.Unmarshal(next(t, a, "welcome"), &w)
	start := w.Players[0]

	clock.Advance(100 * time.Millisecond)
	r.Handle(ctx, a, moveFrame(start.X+500, start.Y))

	var c correctMsg
	_ = json.Unmarshal(next(t, a, "correct"), &c)
	if c.X != start.X || c.Y != start.Y {
		t.Fatalf("correct = %+v, want original position %v,%v", c, start.X, start.Y)
	}
}

func TestSecondTabReplacesFirstAndStaleLeaveIsIgnored(t *testing.T) {
	r, _ := testRoom(t)
	ctx := context.Background()
	tab1, tab2, watcher := client("alice"), client("alice"), client("bob")
	_ = r.Join(ctx, watcher)
	_ = r.Join(ctx, tab1)
	_ = r.Join(ctx, tab2)

	select {
	case <-tab1.Done():
		if code, _ := tab1.CloseInfo(); code != CloseReplaced {
			t.Fatalf("tab1 close code = %d, want %d", code, CloseReplaced)
		}
	case <-time.After(time.Second):
		t.Fatal("tab1 was not kicked")
	}

	// tab1's transport now calls Leave. alice must stay in the room.
	r.Leave(tab1)
	b := client("carol")
	_ = r.Join(ctx, b)
	var w welcomeMsg
	_ = json.Unmarshal(next(t, b, "welcome"), &w)
	if len(w.Players) != 3 {
		t.Fatalf("players = %d, want 3 (alice must survive stale leave)", len(w.Players))
	}
}

func TestLeaveBroadcastsLeft(t *testing.T) {
	r, _ := testRoom(t)
	ctx := context.Background()
	a, b := client("alice"), client("bob")
	_ = r.Join(ctx, a)
	_ = r.Join(ctx, b)
	r.Leave(b)

	var l leftMsg
	_ = json.Unmarshal(next(t, a, "left"), &l)
	if l.ID != "bob" {
		t.Fatalf("left = %q, want bob", l.ID)
	}
}

func TestSlowClientIsKickedNotBlocking(t *testing.T) {
	r, _ := testRoom(t)
	ctx := context.Background()
	slow := client("slow") // never drains Send()
	_ = r.Join(ctx, slow)
	for i := range sendBuffer + 5 {
		_ = r.Join(ctx, client("p"+string(rune('a'+i%26))+string(rune('a'+i/26))))
	}
	select {
	case <-slow.Done():
		if code, _ := slow.CloseInfo(); code != CloseTooSlow {
			t.Fatalf("code = %d, want %d", code, CloseTooSlow)
		}
	case <-time.After(time.Second):
		t.Fatal("slow client was not kicked")
	}
}

func TestProfileNormalize(t *testing.T) {
	cases := []struct {
		in Profile
		ok bool
	}{
		{Profile{Name: "  Him  ", Avatar: Avatar{Color: "#FFB3C6"}}, true},
		{Profile{Name: "", Avatar: Avatar{Color: "#ffb3c6"}}, false},
		{Profile{Name: "abcdefghijklmnopqrstu", Avatar: Avatar{Color: "#ffb3c6"}}, false}, // 21 chars
		{Profile{Name: "bad\x00", Avatar: Avatar{Color: "#ffb3c6"}}, false},
		{Profile{Name: "ok", Avatar: Avatar{Color: "red"}}, false},
	}
	for _, c := range cases {
		got, err := c.in.Normalize()
		if (err == nil) != c.ok {
			t.Errorf("Normalize(%+v) err = %v, want ok=%v", c.in, err, c.ok)
		}
		if c.ok && (got.Name != "Him" || got.Avatar.Color != "#ffb3c6") {
			t.Errorf("Normalize(%+v) = %+v", c.in, got)
		}
	}
}
