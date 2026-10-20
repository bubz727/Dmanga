package descramble

import (
	"encoding/hex"
	"strings"
)

// DecryptComicWalker decrypts Kadokawa Comic-Walker XOR-scrambled WebP images using the drmHash parameter
func DecryptComicWalker(rawBytes []byte, imageURL string) ([]byte, error) {
	idx := strings.Index(imageURL, "#drmHash=")
	if idx == -1 {
		return rawBytes, nil
	}

	drmHash := imageURL[idx+len("#drmHash="):]
	if amp := strings.Index(drmHash, "&"); amp != -1 {
		drmHash = drmHash[:amp]
	}

	if len(drmHash) < 16 {
		return rawBytes, nil
	}

	// First 16 hex chars = 8 bytes key
	keyHex := drmHash[:16]
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) == 0 {
		return rawBytes, nil
	}

	keyLen := len(key)
	dec := make([]byte, len(rawBytes))
	for i := 0; i < len(rawBytes); i++ {
		dec[i] = rawBytes[i] ^ key[i%keyLen]
	}

	return dec, nil
}
