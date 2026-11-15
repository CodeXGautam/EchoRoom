package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// DevVerifier accepts tokens of the form "dev:<id>" with NO signature check.
// It lets you run the whole stack locally without a Supabase project.
// config.LoadConfig refuses to enable it when APP_ENV=production.
type DevVerifier struct{}

var devIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{8,64}$`)

func (DevVerifier) Verify(_ context.Context, token string) (Identity, error) {
	id, ok := strings.CutPrefix(token, "dev:")
	if !ok || !devIDPattern.MatchString(id) {
		return Identity{}, fmt.Errorf("%w: expected dev:<id>", ErrInvalidToken)
	}
	return Identity{UserID: "dev-" + id, IsAnonymous: true}, nil
}
