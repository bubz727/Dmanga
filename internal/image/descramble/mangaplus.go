package descramble

import (
	"encoding/hex"
	"strings"
)

// DecryptMangaPlus decrypts MangaPlus XOR-scrambled images using the encryptionKey parameter
func DecryptMangaPlus(rawBytes []byte, imageURL string) ([]byte, error) {
	idx := strings.Index(imageURL, "#encryptionKey=")
	if idx == -1 {
		return rawBytes, nil
	}

	keyHex := imageURL[idx+len("#encryptionKey="):]
	if amp := strings.Index(keyHex, "&"); amp != -1 {
		keyHex = keyHex[:amp]
	}

	if len(keyHex) < 2 {
		return rawBytes, nil
	}

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
