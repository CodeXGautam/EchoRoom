package room

import "sync"

// CloseCode tells the transport why the room dropped a client. Values in
// 4000-4999 are application-defined WebSocket close codes; the frontend uses
// them to decide whether to reconnect.
type CloseCode int

const (
	CloseShutdown CloseCode = 1001 // server going away: reconnect
	CloseReplaced CloseCode = 4001 // same user opened another tab: don't reconnect
	CloseTooSlow  CloseCode = 4002 // send buffer overflowed: reconnect
	CloseFlooding CloseCode = 4003 // too many messages: reconnect after a pause
)

// sendBuffer is how many outbound frames a client may lag behind before it is
// dropped. At 10 state frames/s this is ~6 s of backlog.
const sendBuffer = 64

// Client is one connected browser tab. The room never touches the network:
// it pushes frames into Send(), and the WebSocket layer drains them.
type Client struct {
	UserID  string
	Profile Profile

	send chan []byte
	done chan struct{}

	once   sync.Once
	code   CloseCode
	reason string
}

func NewClient(userID string, p Profile) *Client {
	return &Client{
		UserID:  userID,
		Profile: p,
		send:    make(chan []byte, sendBuffer),
		done:    make(chan struct{}),
	}
}

// Send yields frames to write to the socket.
func (c *Client) Send() <-chan []byte { return c.send }

// Done is closed when the room (or the transport) wants this client gone.
func (c *Client) Done() <-chan struct{} { return c.done }

// Kick asks the transport to close the connection. Safe to call many times
// and from any goroutine; only the first call's code/reason are kept.
func (c *Client) Kick(code CloseCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.done)
	})
}

// CloseInfo is valid after Done is closed.
func (c *Client) CloseInfo() (CloseCode, string) { return c.code, c.reason }

// trySend never blocks the room loop. A full buffer means the client cannot
// keep up (bad network, background tab); we drop it rather than let one slow
// reader stall everyone else.
func (c *Client) trySend(frame []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- frame:
		return true
	default:
		c.Kick(CloseTooSlow, "too slow")
		return false
	}
}
