package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OIDCConfig struct {
	Issuer      string
	Audience    string
	RoleClaim   string
	RoleMapping map[string]Role
	DefaultRole Role
	Leeway      time.Duration
}

type OIDCAuthenticator struct {
	config OIDCConfig
	client *http.Client

	mu          sync.RWMutex
	refreshMu   sync.Mutex
	keys        map[string]crypto.PublicKey
	jwksURI     string
	lastRefresh time.Time
}

func (*OIDCAuthenticator) Mode() string { return "oidc" }

const minimumRefreshInterval = 30 * time.Second

func NewOIDCAuthenticator(config OIDCConfig, client *http.Client) (*OIDCAuthenticator, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("oidc issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("oidc audience is required")
	}
	if config.RoleClaim == "" {
		config.RoleClaim = "grownerve_role"
	}
	if config.Leeway == 0 {
		config.Leeway = 60 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &OIDCAuthenticator{config: config, client: client, keys: map[string]crypto.PublicKey{}}, nil
}

func (authenticator *OIDCAuthenticator) Authenticate(request *http.Request) (Principal, error) {
	token, found := BearerToken(request)
	if !found {
		return Principal{}, ErrUnauthenticated
	}
	claims, err := authenticator.verify(request.Context(), token)
	if err != nil {
		return Principal{}, err
	}
	role, err := authenticator.roleOf(claims)
	if err != nil {
		return Principal{}, err
	}
	name := claims.Name
	if name == "" {
		name = claims.Email
	}
	if name == "" {
		name = claims.Subject
	}
	return Principal{Subject: claims.Subject, DisplayName: name, Role: role}, nil
}

type oidcClaims struct {
	Issuer    string          `json:"iss"`
	Subject   string          `json:"sub"`
	Audience  json.RawMessage `json:"aud"`
	Expiry    int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
	Name      string          `json:"name"`
	Email     string          `json:"email"`
	raw       map[string]any
}

func (authenticator *OIDCAuthenticator) roleOf(claims *oidcClaims) (Role, error) {
	value, present := claims.raw[authenticator.config.RoleClaim]
	if present {
		for _, candidate := range flattenClaim(value) {
			if mapped, found := authenticator.config.RoleMapping[candidate]; found {
				return mapped, nil
			}
		}
	}
	if authenticator.config.DefaultRole.Valid() {
		return authenticator.config.DefaultRole, nil
	}
	if !present {
		return "", fmt.Errorf("%w: token has no %s claim", ErrUnknownRole, authenticator.config.RoleClaim)
	}
	return "", ErrUnknownRole
}

func flattenClaim(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		var out []string
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

func (authenticator *OIDCAuthenticator) verify(ctx context.Context, token string) (*oidcClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthenticated
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if json.Unmarshal(headerBytes, &header) != nil {
		return nil, ErrUnauthenticated
	}
	if header.Algorithm != "RS256" && header.Algorithm != "RS384" && header.Algorithm != "RS512" &&
		header.Algorithm != "ES256" && header.Algorithm != "ES384" {
		return nil, fmt.Errorf("%w: unsupported token algorithm %q", ErrUnauthenticated, header.Algorithm)
	}
	key, err := authenticator.keyFor(ctx, header.KeyID)
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if err := verifySignature(header.Algorithm, key, []byte(parts[0]+"."+parts[1]), signature); err != nil {
		return nil, err
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrUnauthenticated
	}
	claims := &oidcClaims{}
	if json.Unmarshal(payload, claims) != nil || json.Unmarshal(payload, &claims.raw) != nil {
		return nil, ErrUnauthenticated
	}
	return claims, authenticator.validateClaims(claims)
}

func (authenticator *OIDCAuthenticator) validateClaims(claims *oidcClaims) error {
	now := time.Now()
	if claims.Issuer != authenticator.config.Issuer {
		return fmt.Errorf("%w: unexpected issuer", ErrUnauthenticated)
	}
	if claims.Subject == "" {
		return fmt.Errorf("%w: token has no subject", ErrUnauthenticated)
	}
	if claims.Expiry == 0 || now.After(time.Unix(claims.Expiry, 0).Add(authenticator.config.Leeway)) {
		return fmt.Errorf("%w: token expired", ErrUnauthenticated)
	}
	if claims.NotBefore != 0 && now.Before(time.Unix(claims.NotBefore, 0).Add(-authenticator.config.Leeway)) {
		return fmt.Errorf("%w: token not yet valid", ErrUnauthenticated)
	}
	audiences, err := decodeAudience(claims.Audience)
	if err != nil {
		return err
	}
	for _, audience := range audiences {
		if audience == authenticator.config.Audience {
			return nil
		}
	}
	return fmt.Errorf("%w: token audience does not include this deployment", ErrUnauthenticated)
}

func decodeAudience(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: token has no audience", ErrUnauthenticated)
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return []string{single}, nil
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many, nil
	}
	return nil, fmt.Errorf("%w: unreadable audience claim", ErrUnauthenticated)
}

func verifySignature(algorithm string, key crypto.PublicKey, signed, signature []byte) error {
	var hash crypto.Hash
	switch algorithm {
	case "RS256", "ES256":
		hash = crypto.SHA256
	case "RS384", "ES384":
		hash = crypto.SHA384
	default:
		hash = crypto.SHA512
	}
	var digest []byte
	switch hash {
	case crypto.SHA256:
		sum := sha256.Sum256(signed)
		digest = sum[:]
	case crypto.SHA384:
		sum := sha512.Sum384(signed)
		digest = sum[:]
	default:
		sum := sha512.Sum512(signed)
		digest = sum[:]
	}
	switch typed := key.(type) {
	case *rsa.PublicKey:
		if rsa.VerifyPKCS1v15(typed, hash, digest, signature) != nil {
			return fmt.Errorf("%w: bad token signature", ErrUnauthenticated)
		}
		return nil
	case *ecdsa.PublicKey:
		size := (typed.Curve.Params().BitSize + 7) / 8
		if len(signature) != 2*size {
			return fmt.Errorf("%w: bad token signature", ErrUnauthenticated)
		}
		r := new(big.Int).SetBytes(signature[:size])
		s := new(big.Int).SetBytes(signature[size:])
		if !ecdsa.Verify(typed, digest, r, s) {
			return fmt.Errorf("%w: bad token signature", ErrUnauthenticated)
		}
		return nil
	}
	return fmt.Errorf("%w: unsupported signing key", ErrUnauthenticated)
}
