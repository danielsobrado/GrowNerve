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
	"io"
	"math/big"
	"net/http"
	"net/url"
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
	if err := authenticator.validateFetchedURL(responseURL(response)); err != nil {
		return err
	}
	var document struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := decodeBoundedJSON(response.Body, maximumJWKSBytes, &document); err != nil {
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
	if err := authenticator.validateFetchedURL(responseURL(response)); err != nil {
		return "", err
	}
	var metadata struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := decodeBoundedJSON(response.Body, maximumOIDCMetadataBytes, &metadata); err != nil {
		return "", fmt.Errorf("parse identity provider metadata: %w", err)
	}
	if metadata.Issuer != authenticator.config.Issuer {
		return "", fmt.Errorf("identity provider metadata issuer %q does not match configuration", metadata.Issuer)
	}
	uri, err := authenticator.validateJWKSURI(metadata.JWKSURI)
	if err != nil {
		return "", err
	}
	authenticator.mu.Lock()
	authenticator.jwksURI = uri
	authenticator.mu.Unlock()
	return uri, nil
}

func responseURL(response *http.Response) *url.URL {
	if response == nil || response.Request == nil {
		return nil
	}
	return response.Request.URL
}

func decodeBoundedJSON(reader io.Reader, maximum int64, target any) error {
	data, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maximum {
		return fmt.Errorf("document exceeds %d bytes", maximum)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	return nil
}

func (authenticator *OIDCAuthenticator) validateJWKSURI(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("identity provider metadata has no jwks_uri")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("identity provider jwks_uri is not a valid HTTP or HTTPS URL")
	}
	issuer, _ := url.Parse(authenticator.config.Issuer)
	if issuer.Scheme == "https" && parsed.Scheme != "https" {
		return "", fmt.Errorf("identity provider jwks_uri cannot downgrade an HTTPS issuer")
	}
	return parsed.String(), nil
}

func (authenticator *OIDCAuthenticator) validateFetchedURL(fetched *url.URL) error {
	issuer, _ := url.Parse(authenticator.config.Issuer)
	if issuer.Scheme == "https" && (fetched == nil || fetched.Scheme != "https") {
		return fmt.Errorf("identity provider redirect downgraded HTTPS")
	}
	return nil
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
		if key.Algorithm != "" && key.Algorithm != "RS256" && key.Algorithm != "RS384" && key.Algorithm != "RS512" {
			return nil, fmt.Errorf("key %q has incompatible algorithm %q", key.KeyID, key.Algorithm)
		}
		modulus, err := base64.RawURLEncoding.DecodeString(key.Modulus)
		if err != nil || len(modulus) == 0 {
			return nil, fmt.Errorf("key %q has invalid RSA modulus", key.KeyID)
		}
		exponent, err := base64.RawURLEncoding.DecodeString(key.Exponent)
		if err != nil || len(exponent) == 0 {
			return nil, fmt.Errorf("key %q has invalid RSA exponent", key.KeyID)
		}
		n := new(big.Int).SetBytes(modulus)
		e := new(big.Int).SetBytes(exponent)
		if n.Sign() <= 0 || n.BitLen() < minimumRSABits || e.Sign() <= 0 || e.BitLen() > 31 {
			return nil, fmt.Errorf("key %q has unsafe RSA parameters", key.KeyID)
		}
		exponentValue := e.Int64()
		if exponentValue < 3 || exponentValue%2 == 0 {
			return nil, fmt.Errorf("key %q has unsafe RSA exponent", key.KeyID)
		}
		return &rsa.PublicKey{N: n, E: int(exponentValue)}, nil
	case "EC":
		var curve elliptic.Curve
		switch key.Curve {
		case "P-256":
			if key.Algorithm != "" && key.Algorithm != "ES256" {
				return nil, fmt.Errorf("key %q has incompatible algorithm %q", key.KeyID, key.Algorithm)
			}
			curve = elliptic.P256()
		case "P-384":
			if key.Algorithm != "" && key.Algorithm != "ES384" {
				return nil, fmt.Errorf("key %q has incompatible algorithm %q", key.KeyID, key.Algorithm)
			}
			curve = elliptic.P384()
		default:
			return nil, fmt.Errorf("unsupported curve %q", key.Curve)
		}
		x, err := base64.RawURLEncoding.DecodeString(key.XParam)
		if err != nil || len(x) == 0 {
			return nil, fmt.Errorf("key %q has invalid EC x coordinate", key.KeyID)
		}
		y, err := base64.RawURLEncoding.DecodeString(key.YParam)
		if err != nil || len(y) == 0 {
			return nil, fmt.Errorf("key %q has invalid EC y coordinate", key.KeyID)
		}
		xValue := new(big.Int).SetBytes(x)
		yValue := new(big.Int).SetBytes(y)
		if !curve.IsOnCurve(xValue, yValue) {
			return nil, fmt.Errorf("key %q is not on curve %q", key.KeyID, key.Curve)
		}
		return &ecdsa.PublicKey{Curve: curve, X: xValue, Y: yValue}, nil
	}
	return nil, fmt.Errorf("unsupported key type %q", key.KeyType)
}
