package function

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func Base64Encode(obj interface{}) string {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	err := json.NewEncoder(encoder).Encode(obj)
	if err != nil {
		return ""
	}
	encoder.Close()
	return buf.String()
}

func Base64Decode(obj interface{}, enc string) error {
	return json.NewDecoder(base64.NewDecoder(base64.StdEncoding, strings.NewReader(enc))).Decode(obj)
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
