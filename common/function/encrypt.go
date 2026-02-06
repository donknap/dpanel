package function

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"golang.org/x/crypto/ssh"
)

const (
	CommonKey    = "DPanelCommonAseKey20231208"
	CryptoPrefix = "RSA:"
)

// RSAEncode 统一加密入口：强制使用 RSA 加密
func RSAEncode(str string) (string, error) {
	rsaPubContent, err := os.ReadFile(facade.Config.GetString("system.rsa.pub"))
	if err != nil {
		return "", err
	}
	pubKey, err := RSAParsePublicKey(rsaPubContent)
	if err != nil {
		return "", err
	}
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(str))
	if err != nil {
		return "", err
	}

	return CryptoPrefix + hex.EncodeToString(encrypted), nil
}

// RSADecode 统一解密入口：根据前缀自动切换 RSA 或 AES
func RSADecode(str string, userKey []byte) (string, error) {
	if strings.HasPrefix(str, CryptoPrefix) {
		cipherBytes, err := hex.DecodeString(str[len(CryptoPrefix):])
		if err != nil {
			return "", err
		}
		rsaKeyContent, err := os.ReadFile(facade.Config.GetString("system.rsa.key"))
		if err != nil {
			return "", err
		}
		key, err := RSAParsePrivateKey(rsaKeyContent)
		if err != nil {
			return "", err
		}
		decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, key, cipherBytes)
		if err != nil {
			return "", err
		}
		return string(decrypted), nil
	}
	// 兼容明文的情况
	if userKey == nil {
		return str, nil
	}

	return AseDecode(string(userKey), str)
}

func RSAParsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	privateKey, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		return nil, err
	}
	if v, ok := privateKey.(*rsa.PrivateKey); ok {
		return v, nil
	}
	return nil, errors.New("invalid rsa private key")
}

func RSAParsePublicKey(data []byte) (*rsa.PublicKey, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, err
	}
	if cryptoKey, ok := pub.(ssh.CryptoPublicKey); ok {
		if rsaPub, ok := cryptoKey.CryptoPublicKey().(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
	}
	return nil, errors.New("invalid rsa public key")
}

// Deprecated: AseEncode instead RSAEncode
func AseEncode(key string, str string) (result string, err error) {
	key = Md5(CommonKey + key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	blockSize := block.BlockSize()
	origStr := _PKCS5Padding([]byte(str), blockSize)
	blockMode := cipher.NewCBCEncrypter(block, []byte(key)[:blockSize])
	crypted := make([]byte, len(origStr))
	blockMode.CryptBlocks(crypted, origStr)
	return hex.EncodeToString(crypted), nil
}

// Deprecated: AseDecode instead RSADecode
func AseDecode(key string, str string) (result string, err error) {
	decodeStr, err := hex.DecodeString(str)
	if err != nil {
		return "", err
	}
	key = Md5(CommonKey + key)
	return AesStdDecode(key, decodeStr)
}

func AesStdDecode(key string, originData []byte) (result string, err error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return result, err
	}
	blockSize := block.BlockSize()
	if len(originData) == 0 || len(originData)%blockSize != 0 {
		return "", errors.New("input not full blocks")
	}
	blockMode := cipher.NewCBCDecrypter(block, []byte(key)[:blockSize])
	origData := make([]byte, len(originData))
	blockMode.CryptBlocks(origData, originData)
	origData = _PKCS5UnPadding(origData)
	return string(origData), nil
}

func _PKCS5Padding(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...)
}

func _PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	if length == 0 {
		return nil
	}
	unpadding := int(origData[length-1])
	if length < unpadding {
		return nil
	}
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
			if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') {
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

func Md5(str string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(str)))
}

func Sha256(str []byte) string {
	hash := sha256.New()
	hash.Write(str)
	return fmt.Sprintf("sha256:%x", hash.Sum(nil))
}

func Sha256Struct(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return Sha256(b)
}
