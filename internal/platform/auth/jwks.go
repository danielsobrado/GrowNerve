package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

func (authenticator *OIDCAuthenticator) keyFor(ctx context.Context, keyID string) (crypto.PublicKey, error) {
	authenticator.mu.RLock()
	key, found := authenticator.keys[keyID]
	stale := time.Since(authenticator.lastRefresh) > minimumRefreshInterval
	authenticator.mu.RUnlock()
	if found {
		return key, nil
	}
	if !stale {
		return nil, fmt.Errorf("%w: unknown signing key", ErrUnauthenticated)
	}
	if err := authenticator.refresh(ctx); err != nil {
		return nil, err
	}
	authenticator.mu.RLock()
	defer authenticator.mu.RUnlock()
	if key, found = authenticator.keys[keyID]; found {
		return key, nil
	}
	if keyID == "" && len(authenticator.keys) == 1 {
		for _, only := range authenticator.keys {
			return only, nil
		}
	}
	return nil, fmt.Errorf("%w: unknown signing key", ErrUnauthenticated)
}

func (authenticator *OIDCAuthenticator) refresh(ctx context.Context) error {
	authenticator.refreshMu.Lock()
	defer authenticator.refreshMu.Unlock()

	// Another request may have refreshed while this caller waited for the lock.
	authenticator.mu.RLock()
	recent := !authenticator.lastRefresh.IsZero() && time.Since(authenticator.lastRefresh) <= minimumRefreshInterval
	hasKeys := len(authenticator.keys) > 0
	authenticator.mu.RUnlock()
	if recent && hasKeys {
		return nil
	}

	uri, err := authenticator.discoverJWKS(ctx)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return err
	}
	response, err := authenticator.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch identity provider keys: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("identity provider keys returned %d", response.StatusCode)
	}
	var document struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return fmt.Errorf("parse identity provider keys: %w", err)
	}
	parsed := make(map[string]crypto.PublicKey, len(document.Keys))
	for _, key := range document.Keys {
		public, err := key.publicKey()
		if err != nil {
			continue
		}
		parsed[key.KeyID] = public
	}
	if len(parsed) == 0 {
		return fmt.Errorf("identity provider published no usable keys")
	}
	authenticator.mu.Lock()
	authenticator.keys, authenticator.lastRefresh = parsed, time.Now()
	authenticator.mu.Unlock()
	return nil
}

func (authenticator *OIDCAuthenticator) discoverJWKS(ctx context.Context) (string, error) {
	authenticator.mu.RLock()
	cached := authenticator.jwksURI
	authenticator.mu.RUnlock()
	if cached != "" {
		return cached, nil
	}
	discovery := strings.TrimSuffix(authenticator.config.Issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery, nil)
	if err != nil {
		return "", err
	}
	response, err := authenticator.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch identity provider metadata: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("identity provider metadata returned %d", response.StatusCode)
	}
	var metadata struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("parse identity provider metadata: %w", err)
	}
	if metadata.Issuer != authenticator.config.Issuer {
		return "", fmt.Errorf("identity provider metadata issuer %q does not match configuration", metadata.Issuer)
	}
	if metadata.JWKSURI == "" {
		return "", fmt.Errorf("identity provider metadata has no jwks_uri")
	}
	authenticator.mu.Lock()
	authenticator.jwksURI = metadata.JWKSURI
	authenticator.mu.Unlock()
	return metadata.JWKSURI, nil
}

type jsonWebKey struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Curve     string `json:"crv"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
	XParam    string `json:"x"`
	YParam    string `json:"y"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}

func (key jsonWebKey) publicKey() (crypto.PublicKey, error) {
	if key.Use != "" && key.Use != "sig" {
		return nil, fmt.Errorf("key %q is not a signing key", key.KeyID)
	}
	switch key.KeyType {
	case "RSA":
		modulus, err := base64.RawURLEncoding.DecodeString(key.Modulus)
		if err != nil {
			return nil, err
		}
		exponent, err := base64.RawURLEncoding.DecodeString(key.Exponent)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(new(big.Int).SetBytes(exponent).Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch key.Curve {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		default:
			return nil, fmt.Errorf("unsupported curve %q", key.Curve)
		}
		x, err := base64.RawURLEncoding.DecodeString(key.XParam)
		if err != nil {
			return nil, err
		}
		y, err := base64.RawURLEncoding.DecodeString(key.YParam)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	}
	return nil, fmt.Errorf("unsupported key type %q", key.KeyType)
}
