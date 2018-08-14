package helpers

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateRandomUUID(length int) (string, error) {
	data := make([]byte, length)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(data), nil
}
