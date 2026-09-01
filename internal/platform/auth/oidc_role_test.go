package auth

import (
	"errors"
	"testing"
)

func TestOIDCRoleNamesDoNotGrantPrivilegesWithoutExplicitMapping(t *testing.T) {
	authenticator, err := NewOIDCAuthenticator(OIDCConfig{
		Issuer:      "https://id.example",
		Audience:    "grownerve",
		RoleClaim:   "groups",
		RoleMapping: map[string]Role{"farm-admins": RoleAdministrator},
		DefaultRole: RoleViewer,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	claims := &oidcClaims{raw: map[string]any{"groups": []any{"administrator", "manager", "operator"}}}
	role, err := authenticator.roleOf(claims)
	if err != nil {
		t.Fatal(err)
	}
	if role != RoleViewer {
		t.Fatalf("unmapped literal role names granted %q, want viewer", role)
	}
}

func TestOIDCOnlyMappedClaimCanGrantAdministrator(t *testing.T) {
	authenticator, err := NewOIDCAuthenticator(OIDCConfig{
		Issuer:      "https://id.example",
		Audience:    "grownerve",
		RoleClaim:   "groups",
		RoleMapping: map[string]Role{"farm-admins": RoleAdministrator},
		DefaultRole: RoleViewer,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	claims := &oidcClaims{raw: map[string]any{"groups": []any{"administrator", "farm-admins"}}}
	role, err := authenticator.roleOf(claims)
	if err != nil {
		t.Fatal(err)
	}
	if role != RoleAdministrator {
		t.Fatalf("mapped administrator group resolved to %q", role)
	}
}

func TestOIDCUnmappedClaimFailsClosedWithoutDefaultRole(t *testing.T) {
	authenticator, err := NewOIDCAuthenticator(OIDCConfig{
		Issuer:      "https://id.example",
		Audience:    "grownerve",
		RoleClaim:   "groups",
		RoleMapping: map[string]Role{"farm-admins": RoleAdministrator},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = authenticator.roleOf(&oidcClaims{raw: map[string]any{"groups": "administrator"}})
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("unmapped OIDC role = %v, want ErrUnknownRole", err)
	}
}
