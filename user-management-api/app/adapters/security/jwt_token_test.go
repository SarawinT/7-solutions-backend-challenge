package security

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/SarawinT/user-management-api/app/models/entity"
)

const testSecret = "0123456789abcdef0123456789abcdef" // 32 bytes

func newTestToken(t *testing.T, now time.Time) *JwtToken {
	t.Helper()
	s, err := NewJwtToken(testSecret, "test-issuer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	return s
}

func TestNew_RejectsShortSecret(t *testing.T) {
	if _, err := NewJwtToken("too-short", "iss", time.Hour); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestIssueAndVerify(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newTestToken(t, now)

	token, issued, err := s.Issue("user-1", "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !issued.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %v", issued.ExpiresAt)
	}

	// Header must declare HS256.
	header, _ := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	if !strings.Contains(string(header), `"alg":"HS256"`) {
		t.Fatalf("unexpected header %s", header)
	}

	got, err := s.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "user-1" || got.Email != "alice@example.com" || !got.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatalf("unexpected claims %+v", got)
	}
}

func TestVerify_Rejects(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newTestToken(t, now)
	valid, _, _ := s.Issue("user-1", "alice@example.com")

	otherKey, _ := NewJwtToken("ffffffffffffffffffffffffffffffff", "test-issuer", time.Hour)
	otherKey.now = s.now
	wrongSecret, _, _ := otherKey.Issue("user-1", "alice@example.com")

	otherIss, _ := NewJwtToken(testSecret, "someone-else", time.Hour)
	otherIss.now = s.now
	wrongIssuer, _, _ := otherIss.Issue("user-1", "alice@example.com")

	parts := strings.Split(valid, ".")
	tamperedPayload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"admin","iss":"test-issuer","exp":9999999999}`))
	tampered := parts[0] + "." + tamperedPayload + "." + parts[2]

	// Same secret, different HMAC algorithm: must still be rejected.
	hs512, _ := gojwt.NewWithClaims(gojwt.SigningMethodHS512, gojwt.RegisteredClaims{
		Subject: "user-1", Issuer: "test-issuer",
		ExpiresAt: gojwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString([]byte(testSecret))

	none, _ := gojwt.NewWithClaims(gojwt.SigningMethodNone, gojwt.RegisteredClaims{
		Subject: "user-1", Issuer: "test-issuer",
		ExpiresAt: gojwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString(gojwt.UnsafeAllowNoneSignatureType)

	noExp, _ := gojwt.NewWithClaims(gojwt.SigningMethodHS256, gojwt.RegisteredClaims{
		Subject: "user-1", Issuer: "test-issuer",
	}).SignedString([]byte(testSecret))

	cases := map[string]string{
		"empty":        "",
		"garbage":      "not.a.jwt",
		"wrong secret": wrongSecret,
		"wrong issuer": wrongIssuer,
		"tampered":     tampered,
		"alg HS512":    hs512,
		"alg none":     none,
		"no expiry":    noExp,
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Verify(token); !errors.Is(err, entity.ErrInvalidToken) {
				t.Fatalf("got %v, want ErrInvalidToken", err)
			}
		})
	}

	t.Run("expired", func(t *testing.T) {
		later := newTestToken(t, now.Add(time.Hour+leeway+time.Second))
		if _, err := later.Verify(valid); !errors.Is(err, entity.ErrInvalidToken) {
			t.Fatalf("got %v, want ErrInvalidToken", err)
		}
	})

	t.Run("within leeway", func(t *testing.T) {
		later := newTestToken(t, now.Add(time.Hour+leeway/2))
		if _, err := later.Verify(valid); err != nil {
			t.Fatalf("token within clock-skew leeway rejected: %v", err)
		}
	})
}
