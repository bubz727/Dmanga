package descramble

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"strings"
)

// DecryptMangaUpGlobal decrypts MangaUp Global AES-CBC encrypted images using key and iv fragments
func DecryptMangaUpGlobal(rawBytes []byte, imageURL string) ([]byte, error) {
	idxKey := strings.Index(imageURL, "#key=")
	if idxKey == -1 {
		return rawBytes, nil
	}

	fragment := imageURL[idxKey+len("#key="):]
	parts := strings.Split(fragment, "&iv=")
	if len(parts) < 2 {
		return rawBytes, nil
	}

	keyHex := parts[0]
	ivHex := parts[1]
	if amp := strings.Index(ivHex, "&"); amp != -1 {
		ivHex = ivHex[:amp]
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) == 0 {
		return rawBytes, nil
	}

	iv, err := hex.DecodeString(ivHex)
	if err != nil || len(iv) == 0 {
		return rawBytes, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return rawBytes, nil
	}

	if len(rawBytes)%aes.BlockSize != 0 {
		return rawBytes, nil
	}

	decrypted := make([]byte, len(rawBytes))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, rawBytes)

	// Strip PKCS#7 padding if present
	if len(decrypted) > 0 {
		padLen := int(decrypted[len(decrypted)-1])
		if padLen > 0 && padLen <= aes.BlockSize && padLen <= len(decrypted) {
			isPad := true
			for i := 0; i < padLen; i++ {
				if decrypted[len(decrypted)-1-i] != byte(padLen) {
					isPad = false
					break
				}
			}
			if isPad {
				decrypted = decrypted[:len(decrypted)-padLen]
			}
		}
	}

	return decrypted, nil
}
