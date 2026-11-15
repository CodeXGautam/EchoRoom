package room

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Wire protocol. Every frame in both directions is a JSON envelope:
//
//	{"t": "<type>", "d": <payload>}
//
// Client → server:
//
//	move   {x, y, dir, moving}
//
// Server → client:
//
//	welcome  {selfId, world{w,h}, players[]}    once, right after joining
//	joined   Player                              someone entered
//	left     {id}                                someone left
//	state    {players[]PlayerState}             batched movement, every tick
//	correct  {x, y}                              your last move was rejected; snap here

type envelope struct {
	T string          `json:"t"`
	D json.RawMessage `json:"d"`
}

func encode(t string, d any) []byte {
	b, err := json.Marshal(struct {
		T string `json:"t"`
		D any    `json:"d"`
	}{t, d})
	if err != nil {
		// Only our own structs are encoded here; failure is a programming bug.
		panic("room: encode " + t + ": " + err.Error())
	}
	return b
}

type moveMsg struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Dir    string  `json:"dir"`
	Moving bool    `json:"moving"`
}

type World struct {
	W int `json:"w"`
	H int `json:"h"`
}

type welcomeMsg struct {
	SelfID  string   `json:"selfId"`
	World   World    `json:"world"`
	Players []Player `json:"players"`
}

type leftMsg struct {
	ID string `json:"id"`
}

type stateMsg struct {
	Players []PlayerState `json:"players"`
}

type correctMsg struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Avatar describes how a doodle looks. Phase 1 only has a colour; Phase 4
// adds body, eyes, hat, etc.
type Avatar struct {
	Color string `json:"color"`
}

// Profile is what a user chose on the landing page.
type Profile struct {
	Name   string `json:"name"`
	Avatar Avatar `json:"avatar"`
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const maxNameRunes = 20

var ErrInvalidProfile = errors.New("invalid profile")

// Normalize trims and validates a profile coming from an untrusted client.
func (p Profile) Normalize() (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	n := utf8.RuneCountInString(p.Name)
	if n == 0 || n > maxNameRunes || !utf8.ValidString(p.Name) {
		return Profile{}, errors.Join(ErrInvalidProfile, errors.New("name must be 1-20 characters"))
	}
	for _, r := range p.Name {
		if unicode.IsControl(r) {
			return Profile{}, errors.Join(ErrInvalidProfile, errors.New("name contains control characters"))
		}
	}
	if !colorPattern.MatchString(p.Avatar.Color) {
		return Profile{}, errors.Join(ErrInvalidProfile, errors.New("avatar.color must be #rrggbb"))
	}
	p.Avatar.Color = strings.ToLower(p.Avatar.Color)
	return p, nil
}
