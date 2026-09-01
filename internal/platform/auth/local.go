package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// DevAuthenticator grants a fixed administrator principal without any
// credential. Configuration validation refuses this mode outside development,
// so a production deployment cannot start with authentication disabled.
type DevAuthenticator struct{}

func (DevAuthenticator) Mode() string { return "dev" }

func (DevAuthenticator) Authenticate(*http.Request) (Principal, error) {
	return Principal{Subject: "dev", DisplayName: "Development operator", Role: RoleAdministrator}, nil
}

// LocalAuthenticator checks bearer tokens against hashes supplied through the
// environment. Only the SHA-256 of each token is held, so the configuration
// never contains a usable credential.
type LocalAuthenticator struct{ accounts []localAccount }

type localAccount struct {
	subject string
	role    Role
	digest  [sha256.Size]byte
}

func (LocalAuthenticator) Mode() string { return "local" }

// NewLocalAuthenticator parses "subject:role:sha256hex" entries separated by
// commas or whitespace. It fails loudly on a malformed entry rather than
// silently dropping an account and leaving an endpoint unreachable.
func NewLocalAuthenticator(specification string) (*LocalAuthenticator, error) {
	authenticator := &LocalAuthenticator{}
	for _, entry := range strings.FieldsFunc(specification, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\t' }) {
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("local account %q must be subject:role:sha256", entry)
		}
		role, err := ParseRole(parts[1])
		if err != nil {
			return nil, fmt.Errorf("local account %q: %w", parts[0], err)
		}
		raw, err := hex.DecodeString(strings.TrimSpace(parts[2]))
		if err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("local account %q needs a hex SHA-256 token digest", parts[0])
		}
		account := localAccount{subject: parts[0], role: role}
		copy(account.digest[:], raw)
		authenticator.accounts = append(authenticator.accounts, account)
	}
	if len(authenticator.accounts) == 0 {
		return nil, errors.New("local authentication requires at least one account")
	}
	return authenticator, nil
}

func (authenticator *LocalAuthenticator) Authenticate(request *http.Request) (Principal, error) {
	token, found := BearerToken(request)
	if !found {
		return Principal{}, ErrUnauthenticated
	}
	digest := sha256.Sum256([]byte(token))
	// Every account is compared so a wrong token costs the same time as a right
	// one, and the loop cannot leak which subject nearly matched.
	matched := localAccount{}
	hit := 0
	for _, account := range authenticator.accounts {
		if subtle.ConstantTimeCompare(digest[:], account.digest[:]) == 1 {
			matched, hit = account, 1
		}
	}
	if hit == 0 {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{Subject: matched.subject, DisplayName: matched.subject, Role: matched.role}, nil
}
