package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	jwxjwt "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

const testSecret = "a_very_secure_secret_with_at_least_32_characters_long"

func TestGenerateToken_UsesConfiguredTTL(t *testing.T) {
	InitJWT(&config.Config{JwtSecret: testSecret, JwtAccessTTL: 5 * time.Minute})
	t.Cleanup(func() { InitJWT(&config.Config{JwtSecret: testSecret}) })
	require.Equal(t, 5*time.Minute, AccessTokenTTL())

	token, err := GenerateToken(&model.User{ID: uuid.New(), Role: "user"})
	require.NoError(t, err)
	claims := &Claims{}
	_, _, err = jwt.NewParser().ParseUnverified(token, claims)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, claims.ExpiresAt.Sub(claims.IssuedAt.Time))

	// A zero TTL in config falls back to the default.
	InitJWT(&config.Config{JwtSecret: testSecret})
	assert.Equal(t, config.DefaultJwtAccessTTL, AccessTokenTTL())
}

func TestGenerateToken_RequiresInit(t *testing.T) {
	saved := jwtKey
	jwtKey = nil
	t.Cleanup(func() { jwtKey = saved })

	_, err := GenerateToken(&model.User{ID: uuid.New()})
	assert.Error(t, err)
}

func TestValidateToken_LocalTokenRejections(t *testing.T) {
	InitJWT(&config.Config{JwtSecret: testSecret})
	id := uuid.New()

	sign := func(claims *Claims, method jwt.SigningMethod, key any) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		require.NoError(t, err)
		return s
	}
	valid := func() *Claims {
		return &Claims{ID: id, Role: "user", RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}}
	}
	expired := valid()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	cases := map[string]string{
		"wrong secret": sign(valid(), jwt.SigningMethodHS256, []byte("another_secret_that_is_32_chars_long!!")),
		"expired":      sign(expired, jwt.SigningMethodHS256, []byte(testSecret)),
		"alg none":     sign(valid(), jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType),
		"garbage":      "not-a-jwt",
		"empty":        "",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateToken(token)
			assert.Error(t, err)
		})
	}

	claims, err := ValidateToken(sign(valid(), jwt.SigningMethodHS256, []byte(testSecret)))
	require.NoError(t, err)
	assert.Equal(t, id, claims.ID)
}

// ---- Supabase tokens (verified against a JWKS) ----

// The JWKS cache is initialised once per process (jwksInitOnce), so every
// test shares one fake Supabase key server.
var fakeSupabase = sync.OnceValue(func() jwk.Key {
	raw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	key, err := jwk.FromRaw(raw)
	if err != nil {
		panic(err)
	}
	_ = key.Set(jwk.KeyIDKey, "supabase-test")
	_ = key.Set(jwk.AlgorithmKey, jwa.ES256)

	set := jwk.NewSet()
	pub, _ := key.PublicKey()
	_ = set.AddKey(pub)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	// Never closed: the process-wide JWKS cache keeps this URL.
	InitJWT(&config.Config{JwtSecret: testSecret, SupabaseJWKSURL: srv.URL})
	return key
})

func supabaseToken(t *testing.T, key jwk.Key, edit func(jwxjwt.Token)) string {
	t.Helper()
	tok := jwxjwt.New()
	_ = tok.Set(jwxjwt.SubjectKey, "8f14e45f-ceea-467f-a8f2-6f4e5c6f1a11")
	_ = tok.Set(jwxjwt.AudienceKey, []string{"authenticated"})
	_ = tok.Set(jwxjwt.ExpirationKey, time.Now().Add(time.Hour))
	_ = tok.Set("email", "lan.nguyen@example.com")
	_ = tok.Set("role", "authenticated")
	if edit != nil {
		edit(tok)
	}
	signed, err := jwxjwt.Sign(tok, jwxjwt.WithKey(jwa.ES256, key))
	require.NoError(t, err)
	return string(signed)
}

func TestValidateToken_SupabaseUserToken(t *testing.T) {
	key := fakeSupabase()

	claims, err := ValidateToken(supabaseToken(t, key, nil))
	require.NoError(t, err)
	assert.Equal(t, uuid.MustParse("8f14e45f-ceea-467f-a8f2-6f4e5c6f1a11"), claims.ID)
	assert.Equal(t, "lan.nguyen@example.com", claims.Email)
	assert.Equal(t, "lan.nguyen", claims.Username)
	assert.Equal(t, "user", claims.Role, "Supabase users are never admins here")

	// No email claim: still a user, with empty username.
	claims, err = ValidateToken(supabaseToken(t, key, func(tok jwxjwt.Token) { _ = tok.Remove("email") }))
	require.NoError(t, err)
	assert.Empty(t, claims.Username)
}

func TestValidateToken_SupabaseRejections(t *testing.T) {
	key := fakeSupabase()
	otherRaw, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	otherKey, err := jwk.FromRaw(otherRaw)
	require.NoError(t, err)
	_ = otherKey.Set(jwk.KeyIDKey, "supabase-test") // same kid, different key

	cases := map[string]string{
		"service_role key": supabaseToken(t, key, func(tok jwxjwt.Token) { _ = tok.Set("role", "service_role") }),
		"anon key":         supabaseToken(t, key, func(tok jwxjwt.Token) { _ = tok.Set("role", "anon") }),
		"missing role":     supabaseToken(t, key, func(tok jwxjwt.Token) { _ = tok.Remove("role") }),
		"wrong audience": supabaseToken(t, key, func(tok jwxjwt.Token) {
			_ = tok.Set(jwxjwt.AudienceKey, []string{"anon"})
		}),
		"expired": supabaseToken(t, key, func(tok jwxjwt.Token) {
			_ = tok.Set(jwxjwt.ExpirationKey, time.Now().Add(-time.Minute))
		}),
		"subject not a uuid": supabaseToken(t, key, func(tok jwxjwt.Token) { _ = tok.Set(jwxjwt.SubjectKey, "admin") }),
		"forged signature":   supabaseToken(t, otherKey, nil),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateToken(token)
			assert.Error(t, err)
		})
	}
}
