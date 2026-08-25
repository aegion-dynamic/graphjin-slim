package runtime

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"sync"
)

// gcmCache memoizes AEAD construction per key. Deriving a GCM instance
// (cipher.NewGCM) hashes the key into the GHASH subkey on every call,
// which is pure waste since keys are fixed for an engine's lifetime.
// cipher.AEAD values are documented safe for concurrent use.
var gcmCache sync.Map // [32]byte -> cipher.AEAD

func sharedGCM(key [32]byte) (cipher.AEAD, error) {
	if v, ok := gcmCache.Load(key); ok {
		return v.(cipher.AEAD), nil
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	gcmCache.Store(key, gcm)
	return gcm, nil
}

// EncryptValues encrypts values in data that are prefixed with encPrefix,
// rewriting them with decPrefix and AES-GCM ciphertext.
func EncryptValues(
	data, encPrefix, decPrefix, nonce []byte,
	key [32]byte) ([]byte, error) {
	return encryptValues(data, encPrefix, decPrefix, nonce, key)
}

// DecryptValues decrypts values in data that are prefixed with prefix.
func DecryptValues(data, prefix []byte, key [32]byte) ([]byte, error) {
	return decryptValues(data, prefix, key)
}

// FirstCursorValue returns the first cursor value in data that has a payload
// after the selection-id separator.
func FirstCursorValue(data []byte, prefix []byte) []byte {
	return firstCursorValue(data, prefix)
}

func encryptValues(
	data, encPrefix, decPrefix, nonce []byte,
	key [32]byte) ([]byte, error) {
	var s, e int

	if e = bytes.Index(data[s:], encPrefix); e == -1 {
		return data, nil
	}

	var b bytes.Buffer
	var buf [500]byte

	gcm, err := sharedGCM(key)
	if err != nil {
		return nil, err
	}

	b64 := base64.NewEncoder(base64.RawStdEncoding, &b)

	pl := len(encPrefix)
	nonce = nonce[:gcm.NonceSize()]

	for {
		evs := (s + e + pl)
		q := bytes.IndexByte(data[evs:], '"')
		if q == -1 {
			break
		}
		eve := evs + q
		d := data[evs:eve]
		cl := (len(d) + 64)

		var out []byte
		if cl < len(buf) {
			out = buf[:cl]
		} else {
			out = make([]byte, cl)
		}

		ev := gcm.Seal(
			out[:0],
			nonce,
			d, nil)

		if s == 0 {
			b.Grow(len(data) + (len(data) / 5))
		}
		b.Write(data[s:(s + e)])
		b.Write(decPrefix)
		if _, err := b64.Write(nonce); err != nil {
			return nil, err
		}
		if _, err := b64.Write(ev); err != nil {
			return nil, err
		}
		b64.Close() //nolint:errcheck
		s = eve

		if e = bytes.Index(data[s:], encPrefix); e == -1 {
			break
		}
	}
	b.Write(data[s:])
	return b.Bytes(), nil
}

func decryptValues(data, prefix []byte, key [32]byte) ([]byte, error) {
	var s, e int
	if e = bytes.Index(data[s:], prefix); e == -1 {
		return data, nil
	}

	var b bytes.Buffer
	var buf [500]byte

	gcm, err := sharedGCM(key)
	if err != nil {
		return nil, err
	}

	pl := len(prefix)

	for {
		var fail bool

		evs := e + pl
		q := bytes.IndexByte(data[evs:], '"')
		if q == -1 {
			break
		}
		eve := evs + q
		d := data[evs:eve]
		dl := base64.RawStdEncoding.DecodedLen(len(d))

		var out []byte
		if dl < len(buf) {
			out = buf[:dl]
		} else {
			out = make([]byte, dl)
		}

		_, err := base64.RawStdEncoding.Decode(out, d)
		fail = err != nil

		var out1 []byte
		if !fail {
			out1, err = gcm.Open(
				out[gcm.NonceSize():][:0],
				out[:gcm.NonceSize()],
				out[gcm.NonceSize():],
				nil,
			)
			fail = err != nil
		}

		if s == 0 {
			b.Grow(len(data) + (len(data) / 5))
		}
		b.Write(data[s:e])

		if !fail {
			b.Write(out1)
		} else {
			b.Write(data[e:eve])
		}
		s = eve
		if e = bytes.Index(data[s:], prefix); e == -1 {
			break
		}
		e += s // Convert relative offset to absolute position
	}
	b.Write(data[s:])
	return b.Bytes(), nil
}

func firstCursorValue(data []byte, prefix []byte) []byte {
	s := bytes.Index(data, prefix)
	if s == -1 {
		return nil
	}
	// skip prefix
	i := s + len(prefix)

	// skip alphanumeric digits (sel.ID)
	for i < len(data) && ((data[i] >= '0' && data[i] <= '9') || (data[i] >= 'a' && data[i] <= 'f')) {
		i++
	}

	// Find the end quote
	e := bytes.IndexByte(data[i:], '"')
	if e == -1 {
		return nil
	}
	e = i + e

	if i < len(data) && (data[i] == ',' || data[i] == ':') && (i+1 < e) {
		return data[s:e]
	}
	return nil
}
