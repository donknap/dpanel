package function

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
)

const CommonKey = "DPanelCommonAseKey20231208"

func AseEncode(key string, str string) (result string, err error) {
	key = GetMd5(CommonKey + key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	blockSize := block.BlockSize()
	origStr := PKCS5Padding([]byte(str), blockSize)
	blockMode := cipher.NewCBCEncrypter(block, []byte(key)[:blockSize])
	crypted := make([]byte, len(origStr))
	blockMode.CryptBlocks(crypted, origStr)
	return hex.EncodeToString(crypted), nil
}

func AseDecode(key string, str string) (result string, err error) {
	decodeStr, err := hex.DecodeString(str)
	key = GetMd5(CommonKey + key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, []byte(key)[:blockSize])
	origData := make([]byte, len(decodeStr))
	blockMode.CryptBlocks(origData, decodeStr)
	origData = PKCS5UnPadding(origData)
	return string(origData), nil
}

func PKCS5Padding(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func URIEncodeComponent(s string, excluded ...[]byte) string {
	var b bytes.Buffer
	written := 0
	for i, n := 0, len(s); i < n; i++ {
		c := s[i]
		switch c {
		case '-', '_', '.', '!', '~', '*', '\'', '(', ')':
			continue
		default:
			// Unreserved according to RFC 3986 sec 2.3
			if 'a' <= c && c <= 'z' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
			if len(excluded) > 0 {
				conti := false
				for _, ch := range excluded[0] {
					if ch == c {
						conti = true
						break
					}
				}
				if conti {
					continue
				}
			}
		}
		b.WriteString(s[written:i])
		fmt.Fprintf(&b, "%%%02X", c)
		written = i + 1
	}
	if written == 0 {
		return s
	}
	b.WriteString(s[written:])
	return b.String()
}
