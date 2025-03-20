package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"math/big"
	"strings"
)

func RunSHA512(b []byte) []byte {
	hasher := sha512.New()
	hasher.Write(b)
	return hasher.Sum(nil)
}

var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// GenerateRandomString generates a cryptographically secure random string based on a selection of alphanumerical characters.
func GenerateRandomString(length int) string {
	str := ""
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		if err != nil {
			panic(err)
		}
		str += string(runes[idx.Int64()])
	}
	return str
}

// GenerateRandomBytes generates a cryptographically secure random byte array
func GenerateRandomBytes(length int) []byte {
	b := make([]byte, length)
	// rand.Read fills b with random bytes and never errors according to doc
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Simplified EVP_BytesToKey implementation
// this is used to decrypt V1 metadata
func deriveKeyAndIV(key, salt []byte, keyLen, ivLen int) ([]byte, []byte) {
	keyAndIV := make([]byte, keyLen+ivLen)

	data := make([]byte, 0, 16+len(key))
	for offset := 0; offset < keyLen+ivLen; {
		hash := md5.New()
		hash.Write(data)
		hash.Write(key)
		hash.Write(salt)
		digest := hash.Sum(nil)

		copyLen := min(len(digest), keyLen+ivLen-offset)
		copy(keyAndIV[offset:], digest[:copyLen])
		offset += copyLen

		data = digest
	}

	return keyAndIV[:keyLen], keyAndIV[keyLen:]
}

// V1Decrypt decrypts data using the V1 encryption scheme
func V1Decrypt(data, key []byte) ([]byte, error) {
	// Old and deprecated, not in use anymore, just here for backwards compatibility
	firstBytes := data[:16]
	asciiString := string(firstBytes)
	base64String := base64.StdEncoding.EncodeToString(firstBytes)
	utf8String := string(firstBytes)

	needsConvert := true
	isCBC := true

	if strings.HasPrefix(asciiString, "Salted_") ||
		strings.HasPrefix(base64String, "Salted_") ||
		strings.HasPrefix(utf8String, "Salted_") {
		needsConvert = false
	}

	if strings.HasPrefix(asciiString, "Salted_") ||
		strings.HasPrefix(base64String, "Salted_") ||
		strings.HasPrefix(utf8String, "U2FsdGVk") ||
		strings.HasPrefix(asciiString, "U2FsdGVk") ||
		strings.HasPrefix(utf8String, "Salted_") ||
		strings.HasPrefix(base64String, "U2FsdGVk") {
		isCBC = false
	}

	if needsConvert && !isCBC {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return nil, err
		}
		data = decoded
	}

	if !isCBC {
		saltBytes := data[8:16]

		keyBytes, ivBytes := deriveKeyAndIV([]byte(key), saltBytes, 32, 16)

		block, err := aes.NewCipher(keyBytes)
		if err != nil {
			return nil, err
		}

		mode := cipher.NewCBCDecrypter(block, ivBytes)
		ciphertext := data[16:]
		plaintext := make([]byte, len(ciphertext))
		mode.CryptBlocks(plaintext, ciphertext)

		// Remove PKCS#7 padding
		padding := int(plaintext[len(plaintext)-1])
		return plaintext[:len(plaintext)-padding], nil
	} else {
		keyBytes := []byte(key)
		ivBytes := keyBytes[:16]

		block, err := aes.NewCipher(keyBytes)
		if err != nil {
			return nil, err
		}

		mode := cipher.NewCBCDecrypter(block, ivBytes)
		plaintext := make([]byte, len(data))
		mode.CryptBlocks(plaintext, data)

		// Remove PKCS#7 padding
		padding := int(plaintext[len(plaintext)-1])
		return plaintext[:len(plaintext)-padding], nil
	}
}
