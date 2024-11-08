package protocol

import (
	"encoding/hex"
)

func GetEncryptionKeys(privateKey, publicKey string) ([]byte, []byte, error) {
	targetPublicKeyBytes, err := hex.DecodeString("02" + publicKey)
	if err != nil {
		return nil, nil, err
	}
	privateKeyBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		return nil, nil, err
	}
	return privateKeyBytes, targetPublicKeyBytes, nil
}
