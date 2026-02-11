package steamtotp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const authCodeChars = "23456789BCDFGHJKMNPQRTVWXY"

// GenerateAuthCode generates a 5-character Steam Guard authentication code.
// The sharedSecret is the shared_secret from a Steam maFile (base64 or 40-char hex).
// The timeOffset is the difference between Steam server time and local time in seconds.
func GenerateAuthCode(sharedSecret string, timeOffset int64) (string, error) {
	secret, err := decodeSecret(sharedSecret)
	if err != nil {
		return "", fmt.Errorf("decode shared secret: %w", err)
	}

	timestamp := time.Now().Unix() + timeOffset
	interval := uint64(timestamp / 30)

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], interval)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	hash := mac.Sum(nil)

	// RFC 4226 dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	var result [5]byte
	for i := range result {
		result[i] = authCodeChars[code%uint32(len(authCodeChars))]
		code /= uint32(len(authCodeChars))
	}

	return string(result[:]), nil
}

// decodeSecret decodes a shared secret from either hex or base64 encoding.
func decodeSecret(secret string) ([]byte, error) {
	if len(secret) == 40 {
		if b, err := hex.DecodeString(secret); err == nil {
			return b, nil
		}
	}
	b, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateConfirmationKey generates an HMAC-SHA1 confirmation key.
// The identitySecret is the decoded identity_secret from a Steam maFile.
func GenerateConfirmationKey(identitySecret []byte, timestamp int64, tag string) string {
	buf := make([]byte, 8+len(tag))
	binary.BigEndian.PutUint64(buf[:8], uint64(timestamp))
	copy(buf[8:], tag)

	mac := hmac.New(sha1.New, identitySecret)
	mac.Write(buf)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// GetDeviceID generates a device ID from a SteamID64.
func GetDeviceID(steamID64 uint64) string {
	h := sha1.Sum(fmt.Appendf(nil, "%d", steamID64))
	s := fmt.Sprintf("%x", h)
	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}
