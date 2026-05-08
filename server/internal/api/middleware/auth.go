package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// JWTMiddleware validates Bearer tokens using an HMAC-SHA256 (HS256) secret.
// Suitable for local development and symmetric deployments.
//
// Configure via env vars:
//
//	JWT_SECRET — HMAC secret
//	JWT_ISSUER — expected issuer claim (recommended)
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, ok := extractBearer(r)
			if !ok {
				writeAuthError(w, "missing authorization header")
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				writeAuthError(w, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, token.Claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// JWKSMiddleware validates Bearer tokens using a JWKS endpoint (RS256 / ES256).
// Use this for production OIDC deployments — the issuer's public keys are fetched
// automatically and cached for 5 minutes.
//
//	middleware.JWKSMiddleware(
//	    "https://auth.example.com/.well-known/jwks.json",
//	    "https://auth.example.com",
//	)
func JWKSMiddleware(jwksURL, expectedIssuer string) func(http.Handler) http.Handler {
	cache := &jwksCache{url: jwksURL}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, ok := extractBearer(r)
			if !ok {
				writeAuthError(w, "missing authorization header")
				return
			}

			token, err := jwt.Parse(tokenStr,
				cache.keyFunc,
				jwt.WithIssuer(expectedIssuer),
				jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
			)
			if err != nil || !token.Valid {
				writeAuthError(w, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, token.Claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// jwksCache fetches and caches public keys from a JWKS endpoint with a 5-minute TTL.
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]interface{} // kid → *rsa.PublicKey or *ecdsa.PublicKey
	fetchedAt time.Time
	url       string
}

func (c *jwksCache) keyFunc(t *jwt.Token) (interface{}, error) {
	kid, _ := t.Header["kid"].(string)

	c.mu.RLock()
	if time.Since(c.fetchedAt) < 5*time.Minute && c.keys != nil {
		key := c.keys[kid]
		c.mu.RUnlock()
		if key != nil {
			return key, nil
		}
	} else {
		c.mu.RUnlock()
	}

	// Cache miss or stale — refresh.
	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("jwks: refresh failed: %w", err)
	}

	c.mu.RLock()
	key := c.keys[kid]
	c.mu.RUnlock()

	if key == nil {
		return nil, fmt.Errorf("jwks: no key found for kid %q", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh() error {
	resp, err := http.Get(c.url) //nolint:gosec,noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}

	var set struct {
		Keys []jwkKey `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]interface{}, len(set.Keys))
	for _, k := range set.Keys {
		pub, err := k.publicKey()
		if err != nil {
			continue // skip unsupported key types
		}
		keys[k.Kid] = pub
	}

	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return nil
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	// RSA
	N string `json:"n"`
	E string `json:"e"`
	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwkKey) publicKey() (interface{}, error) {
	switch k.Kty {
	case "RSA":
		return k.rsaPublicKey()
	case "EC":
		return k.ecPublicKey()
	default:
		return nil, fmt.Errorf("unsupported kty: %s", k.Kty)
	}
}

func (k jwkKey) rsaPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, errors.New("jwks: invalid RSA n")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, errors.New("jwks: invalid RSA e")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

func (k jwkKey) ecPublicKey() (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, errors.New("jwks: invalid EC x")
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, errors.New("jwks: invalid EC y")
	}
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("jwks: unsupported curve %s", k.Crv)
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

func extractBearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(h, "Bearer "), true
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}
