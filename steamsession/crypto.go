package steamsession

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

func encryptPassword(password, mod string, exp int64) (string, error) {
	var n big.Int
	n.SetString(mod, 16)

	pubkey := rsa.PublicKey{N: &n, E: int(exp)}
	encPwd, err := rsa.EncryptPKCS1v15(rand.Reader, &pubkey, []byte(password))
	if err != nil {
		return "", fmt.Errorf("rsa encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encPwd), nil
}
