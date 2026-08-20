package core

import "github.com/aegion-dynamic/graphjin-slim/core/v3/runtime"

func encryptValues(
	data, encPrefix, decPrefix, nonce []byte,
	key [32]byte) ([]byte, error) {
	return runtime.EncryptValues(data, encPrefix, decPrefix, nonce, key)
}

func decryptValues(data, prefix []byte, key [32]byte) ([]byte, error) {
	return runtime.DecryptValues(data, prefix, key)
}

func firstCursorValue(data []byte, prefix []byte) []byte {
	return runtime.FirstCursorValue(data, prefix)
}
