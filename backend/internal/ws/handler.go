package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/CodeXGautam/echoroom/internal/auth"
	"github.com/CodeXGautam/echoroom/internal/room"
	"github.com/coder/websocket"
	"golang.org/x/time/rate"
)

const (
	// LobbyID is the single hard-coded room for Phase 1.
	LobbyID = "lobby"

	maxFrameBytes = 4 << 10 // 4 KiB is plenty for move/chat frames
	writeTimeout  = 10 * time.Second
	pingInterval  = 25 * time.Second

	// Per-connection inbound rate limit. Movement is sent at ~15/s.
	msgRate  = 40
	msgBurst = 80
)

// --- POST /api/v1/ws/ticket ---

type ticketRequest struct {
	Profile room.Profile `json:"profile"`
}

type ticketResponse struct {
	Ticket    string `json:"ticket"`
	RoomID    string `json:"roomId"`
	ExpiresIn int    `json:"expiresIn"`
}

func TicketHandler(store *TicketStore, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req ticketRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		profile, err := req.Profile.Normalize()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ticket, err := store.Issue(r.Context(), Claim{
			UserID:    id.UserID,
			Anonymous: id.IsAnonymous,
			RoomID:    LobbyID,
			Profile:   profile,
		})
		if err != nil {
			log.Error("ticket_issue_failed", "err", err)
			http.Error(w, "could not issue ticket", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(ticketResponse{
			Ticket: ticket, RoomID: LobbyID, ExpiresIn: int(ticketTTL.Seconds()),
		})
	}
}

// --- GET /ws?ticket=... ---

type Handler struct {
	Tickets        *TicketStore
	Rooms          *room.Manager
	OriginPatterns []string
	Log            *slog.Logger
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claim, err := h.Tickets.Redeem(r.Context(), r.URL.Query().Get("ticket"))
	if err != nil {
		if !errors.Is(err, ErrTicketInvalid) {
			h.Log.Error("ticket_redeem_failed", "err", err)
		}
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}

	// http.Server's ReadTimeout/WriteTimeout set deadlines on the raw conn.
	// They survive the hijack and would kill every socket after ~10s, so
	// clear them; liveness is handled by pings below instead.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Origin check stops other websites from opening sockets with a
		// stolen ticket (cross-site WebSocket hijacking).
		OriginPatterns: h.OriginPatterns,
	})
	if err != nil {
		return // Accept already wrote the HTTP error
	}
	conn.SetReadLimit(maxFrameBytes)

	h.serve(r.Context(), conn, claim)
}

func (h *Handler) serve(parent context.Context, conn *websocket.Conn, claim Claim) {
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	rm := h.Rooms.Get(claim.RoomID)
	client := room.NewClient(claim.UserID, claim.Profile)
	if err := rm.Join(ctx, client); err != nil {
		conn.Close(websocket.StatusTryAgainLater, "room unavailable")
		return
	}
	defer rm.Leave(client)

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		h.writeLoop(ctx, cancel, conn, client)
	}()

	h.readLoop(ctx, conn, rm, client)
	cancel()
	<-writerDone
}

// readLoop is the only goroutine that reads from conn. It returns when the
// socket errors or closes (including after writeLoop closes it).
func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, rm *room.Room, c *room.Client) {
	limiter := rate.NewLimiter(msgRate, msgBurst)
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if !limiter.Allow() {
			// Don't return: writeLoop sends the close frame, and the close
			// handshake needs this loop still reading.
			c.Kick(room.CloseFlooding, "slow down")
			continue
		}
		if typ != websocket.MessageText {
			continue
		}
		rm.Handle(ctx, c, data)
	}
}

// writeLoop is the only goroutine that writes to conn.
func (h *Handler) writeLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, c *room.Client) {
	defer cancel()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.Done():
			code, reason := c.CloseInfo()
			conn.Close(websocket.StatusCode(code), reason)
			return
		case frame := <-c.Send():
			wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, frame)
			wcancel()
			if err != nil {
				return
			}
		case <-ping.C:
			// Ping needs readLoop running to receive the pong, which it is.
			pctx, pcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}
