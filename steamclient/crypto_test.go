package steamclient

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		t.Fatalf("generate session key: %v", err)
	}

	cipher, err := newChannelCipher(sessionKey, true)
	if err != nil {
		t.Fatalf("newChannelCipher: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"exact block", bytes.Repeat([]byte{0xAB}, 16)},
		{"multi block", bytes.Repeat([]byte{0xCD}, 100)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := cipher.encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}

			decrypted, err := cipher.decrypt(encrypted)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}

			if !bytes.Equal(decrypted, tc.plaintext) {
				t.Errorf("round-trip mismatch: got %x, want %x", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	sessionKey := make([]byte, 32)
	rand.Read(sessionKey)

	cipher, err := newChannelCipher(sessionKey, true)
	if err != nil {
		t.Fatalf("newChannelCipher: %v", err)
	}

	plaintext := []byte("same input")

	enc1, _ := cipher.encrypt(plaintext)
	enc2, _ := cipher.encrypt(plaintext)

	// Due to random IV component, encryptions should differ
	if bytes.Equal(enc1, enc2) {
		t.Error("two encryptions of same plaintext produced identical output")
	}
}

func TestPKCS7PadUnpad(t *testing.T) {
	for _, size := range []int{0, 1, 15, 16, 17, 31, 32} {
		data := make([]byte, size)
		padded := pkcs7Pad(data, 16)

		if len(padded)%16 != 0 {
			t.Errorf("size=%d: padded length %d not block-aligned", size, len(padded))
		}

		unpadded, err := pkcs7Unpad(padded, 16)
		if err != nil {
			t.Errorf("size=%d: unpad error: %v", size, err)
			continue
		}

		if !bytes.Equal(unpadded, data) {
			t.Errorf("size=%d: pad/unpad round-trip mismatch", size)
		}
	}
}

func TestInvalidSessionKeyLength(t *testing.T) {
	_, err := newChannelCipher([]byte("too short"), true)
	if err == nil {
		t.Error("expected error for invalid session key length")
	}
}
