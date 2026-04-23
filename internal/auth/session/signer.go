package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Signer creates and validates HMAC-signed session tokens containing a user ID
// and expiration timestamp.
type Signer struct {
	key []byte
}

func NewSigner(secret, scope string) *Signer {
	if secret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("failed to generate session key: " + err.Error())
		}
		slog.Warn(scope + ": no session secret configured — using random key (sessions will not survive restarts)")
		return &Signer{key: b}
	}
	return &Signer{key: []byte(secret)}
}

// CreateToken returns a token in the format "userID:expiry:signature".
func (s *Signer) CreateToken(userID int64, ttl time.Duration) string {
	expiry := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	uid := strconv.FormatInt(userID, 10)
	payload := uid + ":" + expiry
	return payload + ":" + s.sign(payload)
}

// Validate checks that the token is well-formed, not expired, and correctly
// signed. It returns the user ID on success.
func (s *Signer) Validate(token string) (int64, bool) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return 0, false
	}
	uidStr, expiryStr, sig := parts[0], parts[1], parts[2]

	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return 0, false
	}

	userID, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return 0, false
	}

	expected := s.sign(uidStr + ":" + expiryStr)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return 0, false
	}
	return userID, true
}

func (s *Signer) sign(data string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
