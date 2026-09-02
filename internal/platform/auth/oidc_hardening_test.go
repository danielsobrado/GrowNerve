package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestOIDCRejectsInvalidIssuerAndOversizedToken(t *testing.T) {
	for _, issuer := range []string{"id.example", "ftp://id.example", "https://user@id.example", "https://id.example?tenant=x"} {
		if _, err := NewOIDCAuthenticator(OIDCConfig{Issuer: issuer, Audience: "grownerve"}, nil); err == nil {
			t.Fatalf("invalid issuer %q was accepted", issuer)
		}
	}
	authenticator, err := NewOIDCAuthenticator(OIDCConfig{Issuer: "https://id.example", Audience: "grownerve"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.verify(t.Context(), strings.Repeat("x", maximumOIDCTokenBytes+1)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("oversized token error = %v, want ErrUnauthenticated", err)
	}
}

func TestOIDCRejectsJWKSHTTPSDowngradeAndOversizedDocuments(t *testing.T) {
	authenticator, err := NewOIDCAuthenticator(OIDCConfig{Issuer: "https://id.example", Audience: "grownerve"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.validateJWKSURI("http://keys.example/jwks"); err == nil {
		t.Fatal("HTTPS issuer accepted an HTTP jwks_uri")
	}
	if _, err := authenticator.validateJWKSURI("https://keys.example/jwks"); err != nil {
		t.Fatalf("valid HTTPS jwks_uri rejected: %v", err)
	}
	var target map[string]any
	if err := decodeBoundedJSON(strings.NewReader(strings.Repeat(" ", maximumOIDCMetadataBytes+1)), maximumOIDCMetadataBytes, &target); err == nil {
		t.Fatal("oversized OIDC document was accepted")
	}
}

func TestJWKRejectsWeakRSAAndInvalidECPoint(t *testing.T) {
	weakModulus := make([]byte, 128)
	for index := range weakModulus {
		weakModulus[index] = 0xff
	}
	weakRSA := jsonWebKey{
		KeyType: "RSA", KeyID: "weak", Algorithm: "RS256",
		Modulus: base64.RawURLEncoding.EncodeToString(weakModulus),
		Exponent: base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
	}
	if _, err := weakRSA.publicKey(); err == nil {
		t.Fatal("1024-bit RSA key was accepted")
	}

	invalidEC := jsonWebKey{
		KeyType: "EC", KeyID: "bad-point", Curve: "P-256", Algorithm: "ES256",
		XParam: base64.RawURLEncoding.EncodeToString([]byte{1}),
		YParam: base64.RawURLEncoding.EncodeToString([]byte{1}),
	}
	if _, err := invalidEC.publicKey(); err == nil {
		t.Fatal("off-curve EC key was accepted")
	}
}
