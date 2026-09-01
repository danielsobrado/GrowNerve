// Package auth establishes who is making a request. Authorization decisions
// live with the resources they protect; this package only proves identity and
// carries the resulting principal on the request context.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Role is a coarse capability tier. Ranks are ordered so a check can ask for a
// minimum tier rather than enumerating every role.
type Role string

const (
	RoleViewer        Role = "viewer"
	RoleOperator      Role = "operator"
	RoleManager       Role = "manager"
	RoleAdministrator Role = "administrator"
)

var roleRank = map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleManager: 3, RoleAdministrator: 4}

// Valid reports whether the role is one this system recognises.
func (role Role) Valid() bool { _, found := roleRank[role]; return found }

// AtLeast reports whether the role meets a minimum tier.
func (role Role) AtLeast(minimum Role) bool { return roleRank[role] >= roleRank[minimum] }

// ParseRole converts external role text, rejecting anything unrecognised rather
// than defaulting to a permissive value.
func ParseRole(value string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(value)))
	if !role.Valid() {
		return "", ErrUnknownRole
	}
	return role, nil
}

// Principal is an authenticated caller.
type Principal struct {
	Subject     string
	DisplayName string
	Role        Role
}

var (
	// ErrUnauthenticated reports a missing or unreadable credential.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrUnknownRole reports a role value outside the supported set.
	ErrUnknownRole = errors.New("unknown role")
)

type contextKey struct{}

// WithPrincipal returns a context carrying the authenticated caller.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

// PrincipalFrom recovers the authenticated caller, reporting whether one was
// established. A false result must never be treated as permission.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, found := ctx.Value(contextKey{}).(Principal)
	return principal, found
}

// Authenticator resolves a request to a principal.
type Authenticator interface {
	Authenticate(request *http.Request) (Principal, error)
	// Mode names the configured strategy, for logging and startup validation.
	Mode() string
}

// BearerToken extracts a bearer credential from the Authorization header.
func BearerToken(request *http.Request) (string, bool) {
	header := request.Header.Get("Authorization")
	if len(header) < 7 || !strings.EqualFold(header[:7], "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(header[7:])
	return token, token != ""
}
