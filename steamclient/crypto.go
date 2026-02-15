package steamclient

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"fmt"
)

const (
	ivLen       = 16 // AES block size
	ivRandomLen = 3  // random bytes appended to HMAC hash in IV
)

// channelCipher implements Steam's TCP channel encryption.
// With useHMAC=true it matches SteamKit's NetFilterEncryptionWithHMAC
// (HMAC-SHA1-derived IVs). With useHMAC=false it matches NetFilterEncryption
// (random IVs).
type channelCipher struct {
	block      cipher.Block
	hmacSecret []byte // first 16 bytes of sessionKey (only used when useHMAC is true)
	useHMAC    bool
}

func newChannelCipher(sessionKey []byte, useHMAC bool) (*channelCipher, error) {
	if len(sessionKey) != 32 {
		return nil, fmt.Errorf("session key must be 32 bytes, got %d", len(sessionKey))
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}

	var hmacKey []byte
	if useHMAC {
		hmacKey = make([]byte, 16)
		copy(hmacKey, sessionKey[:16])
	}

	return &channelCipher{
		block:      block,
		hmacSecret: hmacKey,
		useHMAC:    useHMAC,
	}, nil
}

// encrypt encrypts plaintext using AES-256-CBC.
//
// With HMAC mode: generate 3 random bytes, compute HMAC-SHA1(hmacSecret, random3 + plaintext),
// take first 13 bytes of hash + the 3 random bytes = 16-byte IV.
// Without HMAC: generate 16 random bytes as IV.
// Output: AES-ECB(IV) + AES-CBC(plaintext, IV) with PKCS7 padding.
func (c *channelCipher) encrypt(plaintext []byte) ([]byte, error) {
	iv := make([]byte, ivLen)

	if c.useHMAC {
		// 3 random bytes go at the end
		if _, err := rand.Read(iv[ivLen-ivRandomLen:]); err != nil {
			return nil, fmt.Errorf("rand.Read: %w", err)
		}
		// HMAC-SHA1(hmacSecret, random3 + plaintext)
		mac := hmac.New(sha1.New, c.hmacSecret)
		mac.Write(iv[ivLen-ivRandomLen:])
		mac.Write(plaintext)
		hash := mac.Sum(nil)
		copy(iv[:ivLen-ivRandomLen], hash[:ivLen-ivRandomLen])
	} else {
		if _, err := rand.Read(iv); err != nil {
			return nil, fmt.Errorf("rand.Read: %w", err)
		}
	}

	encryptedIV := make([]byte, ivLen)
	c.block.Encrypt(encryptedIV, iv)

	padded := pkcs7Pad(plaintext, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(c.block, iv)
	mode.CryptBlocks(ciphertext, padded)

	out := make([]byte, ivLen+len(ciphertext))
	copy(out, encryptedIV)
	copy(out[ivLen:], ciphertext)
	return out, nil
}

// decrypt decrypts data encrypted by encrypt.
func (c *channelCipher) decrypt(data []byte) ([]byte, error) {
	if len(data) < ivLen+aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short: %d bytes", len(data))
	}

	iv := make([]byte, ivLen)
	c.block.Decrypt(iv, data[:ivLen])

	cbcData := data[ivLen:]
	if len(cbcData)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext not block-aligned: %d bytes", len(cbcData))
	}

	plaintext := make([]byte, len(cbcData))
	mode := cipher.NewCBCDecrypter(c.block, iv)
	mode.CryptBlocks(plaintext, cbcData)

	plaintext, err := pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("pkcs7 unpad: %w", err)
	}

	if c.useHMAC {
		mac := hmac.New(sha1.New, c.hmacSecret)
		mac.Write(iv[ivLen-ivRandomLen:]) // random3
		mac.Write(plaintext)
		expectedHash := mac.Sum(nil)

		if !hmac.Equal(iv[:ivLen-ivRandomLen], expectedHash[:ivLen-ivRandomLen]) {
			return nil, fmt.Errorf("HMAC verification failed")
		}
	}

	return plaintext, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padded data length: %d", len(data))
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, fmt.Errorf("invalid padding value: %d", padding)
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding byte at position %d", i)
		}
	}
	return data[:len(data)-padding], nil
}

// rsaEncryptSessionKey encrypts the session key (and optional challenge) with
// Steam's RSA public key using OAEP-SHA1.
func rsaEncryptSessionKey(sessionKey, challenge []byte) ([]byte, error) {
	pub, err := x509.ParsePKIXPublicKey(steamPublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	blob := sessionKey
	if challenge != nil {
		blob = make([]byte, len(sessionKey)+len(challenge))
		copy(blob, sessionKey)
		copy(blob[len(sessionKey):], challenge)
	}

	return rsa.EncryptOAEP(sha1.New(), rand.Reader, rsaPub, blob, nil)
}

// steamPublicKey is Steam's RSA public key for the Public universe (DER-encoded PKIX).
// From SteamKit2/Util/KeyDictionary.cs — EUniverse.Public.
var steamPublicKey = []byte{
	0x30, 0x81, 0x9D, 0x30, 0x0D, 0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x01, 0x01,
	0x05, 0x00, 0x03, 0x81, 0x8B, 0x00, 0x30, 0x81, 0x87, 0x02, 0x81, 0x81, 0x00, 0xDF, 0xEC, 0x1A,
	0xD6, 0x2C, 0x10, 0x66, 0x2C, 0x17, 0x35, 0x3A, 0x14, 0xB0, 0x7C, 0x59, 0x11, 0x7F, 0x9D, 0xD3,
	0xD8, 0x2B, 0x7A, 0xE3, 0xE0, 0x15, 0xCD, 0x19, 0x1E, 0x46, 0xE8, 0x7B, 0x87, 0x74, 0xA2, 0x18,
	0x46, 0x31, 0xA9, 0x03, 0x14, 0x79, 0x82, 0x8E, 0xE9, 0x45, 0xA2, 0x49, 0x12, 0xA9, 0x23, 0x68,
	0x73, 0x89, 0xCF, 0x69, 0xA1, 0xB1, 0x61, 0x46, 0xBD, 0xC1, 0xBE, 0xBF, 0xD6, 0x01, 0x1B, 0xD8,
	0x81, 0xD4, 0xDC, 0x90, 0xFB, 0xFE, 0x4F, 0x52, 0x73, 0x66, 0xCB, 0x95, 0x70, 0xD7, 0xC5, 0x8E,
	0xBA, 0x1C, 0x7A, 0x33, 0x75, 0xA1, 0x62, 0x34, 0x46, 0xBB, 0x60, 0xB7, 0x80, 0x68, 0xFA, 0x13,
	0xA7, 0x7A, 0x8A, 0x37, 0x4B, 0x9E, 0xC6, 0xF4, 0x5D, 0x5F, 0x3A, 0x99, 0xF9, 0x9E, 0xC4, 0x3A,
	0xE9, 0x63, 0xA2, 0xBB, 0x88, 0x19, 0x28, 0xE0, 0xE7, 0x14, 0xC0, 0x42, 0x89, 0x02, 0x01, 0x11,
}
