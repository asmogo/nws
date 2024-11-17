package protocol

import (
	"encoding/hex"
	"fmt"
)

const (
	padding = "02"
)

func GetEncryptionKeys(privateKey, publicKey string) ([]byte, []byte, error) {
	targetPublicKeyBytes, err := hex.DecodeString(padding + publicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode private key: %w", err)
	}
	return privateKeyBytes, targetPublicKeyBytes, nil
}
