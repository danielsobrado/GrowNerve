package farm

import (
	"errors"
	"net/http"

	"github.com/jdanielsobrado/grownerve/internal/platform/auth"
)

// Actions this API protects. Authorization is deliberately separate from safety
// validation: a caller may be permitted to request an actuator change and still
// have it refused by an interlock.
const (
	ActionRead         = "farm.read"
	ActionWriteState   = "farm.write_state"
	ActionIssueCommand = "command.issue"
	ActionObserve      = "observation.write"
	ActionAdminister   = "system.administer"
)

// minimumRole is the least privileged tier permitted to perform each action.
// Replacing the whole farm document is configuration editing, so it requires a
// manager even though issuing a low-risk command only requires an operator.
var minimumRole = map[string]auth.Role{
	ActionRead:         auth.RoleViewer,
	ActionWriteState:   auth.RoleManager,
	ActionIssueCommand: auth.RoleOperator,
	ActionObserve:      auth.RoleOperator,
	ActionAdminister:   auth.RoleAdministrator,
}

// ErrForbidden reports an authenticated caller whose role is too low.
var ErrForbidden = errors.New("forbidden")

// RoleAuthorizer enforces the action table against the principal established by
// the authentication middleware. An action with no entry is refused rather than
// allowed, so adding an endpoint cannot accidentally open it to everyone.
type RoleAuthorizer struct{}

func (RoleAuthorizer) Authorize(request *http.Request, action string) error {
	principal, found := auth.PrincipalFrom(request.Context())
	if !found {
		return auth.ErrUnauthenticated
	}
	required, known := minimumRole[action]
	if !known {
		return ErrForbidden
	}
	if !principal.Role.AtLeast(required) {
		return ErrForbidden
	}
	return nil
}

// ActorOf names the caller for audit records, falling back to a stable label
// when no principal was established.
func ActorOf(request *http.Request) string {
	if principal, found := auth.PrincipalFrom(request.Context()); found {
		return principal.Subject
	}
	return "anonymous"
}

func writeAuthorizationProblem(writer http.ResponseWriter, request *http.Request, err error) {
	if errors.Is(err, auth.ErrUnauthenticated) {
		writer.Header().Set("WWW-Authenticate", `Bearer realm="grownerve"`)
		writeProblem(writer, request, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in to use this endpoint")
		return
	}
	writeProblem(writer, request, http.StatusForbidden, "FORBIDDEN", "Your role does not permit this action")
}
