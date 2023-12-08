package function

import (
	"crypto/aes"
	"encoding/hex"
	"strings"
)

const COMMON_KEY = "DPanelCommonAseKey20231208"

func AseEncode(key string, str string) (result string, err error) {
	key = GetMd5(COMMON_KEY + key)
	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	strLen := len(str)
	for i := 0; i < 16-strLen%16; i++ {
		str += "\n"
	}
	out := make([]byte, len(str))
	cipher.Encrypt(out, []byte(str))
	return hex.EncodeToString(out), nil
}

func AseDecode(key string, str string) (result string, err error) {
	decodeStr, err := hex.DecodeString(str)
	key = GetMd5(COMMON_KEY + key)
	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	out := make([]byte, len(decodeStr))
	cipher.Decrypt(out, decodeStr)
	return strings.Trim(string(out), "\n"), nil
}
