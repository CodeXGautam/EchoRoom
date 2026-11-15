package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/CodeXGautam/echoroom/internal/room"
	"github.com/redis/go-redis/v9"
)

// Why tickets: browsers cannot set an Authorization header on a WebSocket,
// and putting the long-lived JWT in the URL leaks it into proxy/access logs.
// So the client trades its JWT (in a normal header) for a random ticket that
// is valid once, for a few seconds, and only carries what the socket needs.

const ticketTTL = 30 * time.Second

var ErrTicketInvalid = errors.New("ticket invalid or expired")

type Claim struct {
	UserID    string       `json:"uid"`
	Anonymous bool         `json:"anon"`
	RoomID    string       `json:"room"`
	Profile   room.Profile `json:"profile"`
}

type TicketStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewTicketStore(rdb *redis.Client) *TicketStore {
	return &TicketStore{rdb: rdb, ttl: ticketTTL}
}

func ticketKey(id string) string { return "ws:ticket:" + id }

func (s *TicketStore) Issue(ctx context.Context, c Claim) (string, error) {
	buf := make([]byte, 32) // 256 bits: unguessable
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate ticket: %w", err)
	}
	id := base64.RawURLEncoding.EncodeToString(buf)

	payload, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode claim: %w", err)
	}
	if err := s.rdb.Set(ctx, ticketKey(id), payload, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("store ticket: %w", err)
	}
	return id, nil
}

// Redeem returns the claim and deletes the ticket atomically (GETDEL), so a
// ticket can never be used twice, even by two servers racing.
func (s *TicketStore) Redeem(ctx context.Context, id string) (Claim, error) {
	if id == "" || len(id) > 64 {
		return Claim{}, ErrTicketInvalid
	}
	raw, err := s.rdb.GetDel(ctx, ticketKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Claim{}, ErrTicketInvalid
	}
	if err != nil {
		return Claim{}, fmt.Errorf("redeem ticket: %w", err)
	}
	var c Claim
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claim{}, fmt.Errorf("decode claim: %w", err)
	}
	return c, nil
}
