// Package crypto provides the cryptographic functions required within the SDK.
//
// There are two kinds of decrypted data:
//   - Metadata means any small string data, typically file metadata, but also e.g. directory names.
//   - Data means file content.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
	"slices"
	"strings"
)

type MetaCrypter interface {
	EncryptMeta(metadata string) EncryptedString
	DecryptMeta(encrypted EncryptedString) (string, error)
}

// EncryptedString denotes that a string is encrypted and can't be used meaningfully before being decrypted.
type EncryptedString string

// NewEncryptedStringV2 creates a new EncryptedString with the v2 format
func NewEncryptedStringV2(encrypted []byte, nonce [12]byte) EncryptedString {
	return EncryptedString("002" + string(nonce[:]) + base64.StdEncoding.EncodeToString(encrypted))
}

// NewEncryptedStringV3 creates a new EncryptedString with the v3 format
func NewEncryptedStringV3(encrypted []byte, nonce [12]byte) EncryptedString {
	return EncryptedString("003" + hex.EncodeToString(nonce[:]) + base64.StdEncoding.EncodeToString(encrypted))
}

// MasterKeys is a slice of MasterKey, this is used by the V1 and V2 encryption schemes
type MasterKeys []MasterKey

// NewMasterKeys creates a new MasterKeys slice
func NewMasterKeys(encryptionKey MasterKey, stringKeys string) (MasterKeys, error) {
	keys := make([]MasterKey, 0)
	for _, key := range strings.Split(stringKeys, "|") {
		if len(key) != 64 {
			return nil, fmt.Errorf("key length wrong %d", len(key))
		}
		keyBytes := []byte(key)
		keySized := [64]byte(keyBytes)
		mk, err := NewMasterKey(keySized)
		if err != nil {
			return nil, fmt.Errorf("NewMasterKey: %w", err)
		}
		if encryptionKey.DerivedBytes == mk.DerivedBytes {
			continue
		}
		keys = append(keys, *mk)
	}
	keys = slices.Insert(keys, 0, encryptionKey)
	return keys, nil
}

func getCipherForKey(key [32]byte) (cipher.AEAD, error) {
	c, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("getCipherForKey: %v", err)
	}
	derivedGcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("getCipherForKey: %v", err)
	}
	return derivedGcm, nil
}

// DecryptMeta should be avoided, and Filen.DecryptMeta should be used instead,
// but this is necessary for RSA Keypair decryption
func (ms *MasterKeys) DecryptMeta(encrypted EncryptedString) (string, error) {
	if encrypted[0:8] == "U2FsdGVk" {
		return ms.DecryptMetaV1(encrypted)
	}
	if encrypted[0:3] == "002" {
		return ms.DecryptMetaV2(encrypted)
	}
	return "", fmt.Errorf("unknown metadata format")
}

// MasterKey is a key used to encrypt and decrypt metadata
// in the v1 and v2 encryption schemes
type MasterKey struct {
	Bytes        [64]byte
	DerivedBytes [32]byte
	cipher       cipher.AEAD
}

// NewMasterKey creates a new MasterKey from a 64 byte key
// usually this is a string of 64 characters
func NewMasterKey(key [64]byte) (*MasterKey, error) {
	derivedKey := pbkdf2.Key(key[:], key[:], 1, 32, sha512.New)
	derivedBytes := [32]byte{}
	copy(derivedBytes[:], derivedKey[:32])
	c, err := getCipherForKey(derivedBytes)
	if err != nil {
		return nil, fmt.Errorf("NewMasterKey: %v", err)
	}
	return &MasterKey{
		Bytes:        key,
		DerivedBytes: derivedBytes,
		cipher:       c,
	}, nil
}

// EncryptMeta should be avoided, and Filen.EncryptMeta should be used instead
func (m *MasterKey) EncryptMeta(metadata string) EncryptedString {
	nonce := [12]byte([]byte(GenerateRandomString(12)))
	encrypted := m.cipher.Seal(nil, nonce[:], []byte(metadata), nil)
	return NewEncryptedStringV2(encrypted, nonce)
}

func (m *MasterKey) decryptMetaV1(metadata EncryptedString) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(metadata))
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	salt := decoded[8:16]
	cipherText := decoded[16:]

	keyBytes, ivBytes := deriveKeyAndIV(m.DerivedBytes[:], salt, 32, 16)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, ivBytes)

	plaintext := make([]byte, len(cipherText))
	mode.CryptBlocks(plaintext, cipherText)

	paddingLen := int(plaintext[len(plaintext)-1])
	if paddingLen > aes.BlockSize || paddingLen <= 0 {
		return "", fmt.Errorf("invalid padding size")
	}

	return string(plaintext[:len(plaintext)-paddingLen]), nil
}

// DecryptMetaV2 should be avoided, and Filen.DecryptMeta should be used instead
func (m *MasterKey) DecryptMetaV2(metadata EncryptedString) (string, error) {
	nonce := metadata[3:15]
	decoded, err := base64.StdEncoding.DecodeString(string(metadata[15:]))
	if err != nil {
		return "", fmt.Errorf("DecryptMetadataV2: %v", err)
	}
	decoded, err = m.cipher.Open(decoded[:0], []byte(nonce), decoded, nil)
	if err != nil {
		return "", fmt.Errorf("DecryptMetadataV2: %v", err)
	}
	return string(decoded), nil
}

// DecryptMeta should be avoided, and Filen.DecryptMeta should be used instead
func (m *MasterKey) DecryptMeta(metadata EncryptedString) (string, error) {
	if metadata[0:8] == "U2FsdGVk" {
		return m.decryptMetaV1(metadata)
	}
	switch metadata[0:3] {
	case "002":
		return m.DecryptMetaV2(metadata)
	default:
		return "", fmt.Errorf("unknown metadata format")
	}
}

// AllKeysFailedError denotes that no key passed to [DecryptMetadataAllKeys] worked.
type AllKeysFailedError struct {
	Errors []error // errors thrown in the process
}

func (e *AllKeysFailedError) Error() string {
	return fmt.Sprintf("all keys failed: %v", e.Errors)
}

func (ms *MasterKeys) decryptMeta(metadata EncryptedString, decryptFunc func(m *MasterKey, encryptedString EncryptedString) (string, error)) (string, error) {
	errs := make([]error, 0)
	for _, masterKey := range *ms {
		var decrypted string
		decrypted, err := decryptFunc(&masterKey, metadata)
		if err == nil {
			return decrypted, nil
		}
		errs = append(errs, err)
	}
	return "", &AllKeysFailedError{Errors: errs}
}

// DecryptMetaV1 should be avoided, and Filen.DecryptMeta should be used instead
func (ms *MasterKeys) DecryptMetaV1(metadata EncryptedString) (string, error) {
	return ms.decryptMeta(metadata, (*MasterKey).decryptMetaV1)
}

// DecryptMetaV2 should be avoided, and Filen.DecryptMeta should be used instead
func (ms *MasterKeys) DecryptMetaV2(metadata EncryptedString) (string, error) {
	return ms.decryptMeta(metadata, (*MasterKey).DecryptMetaV2)
}

// EncryptMeta should be avoided, and Filen.EncryptMeta should be used instead
func (ms *MasterKeys) EncryptMeta(metadata string) EncryptedString {
	// potential null dereference which makes me uncomfortable
	// this function should only ever be called on non-empty MasterKeys
	// which should be safe since in v2 there must be at least 1 master key,
	// and in v3 we won't be using this function
	return (*ms)[0].EncryptMeta(metadata)
}

// DerivedPassword is derived from the user password, and used to authenticate the user to the backend
type DerivedPassword string

// DeriveMKAndAuthFromPassword returns a MasterKey and a DerivedPassword
func DeriveMKAndAuthFromPassword(password string, salt string) (*MasterKey, DerivedPassword, error) {
	// makes a 128 byte string
	derived := hex.EncodeToString(pbkdf2.Key([]byte(password), []byte(salt), 200000, 64, sha512.New))
	var (
		rawMasterKey [64]byte
	)
	copy(rawMasterKey[:], derived[:64])

	hasher := sha512.New()
	hasher.Write([]byte(derived[64:])) // write password
	derivedPass := DerivedPassword(hex.EncodeToString(hasher.Sum(nil)))

	masterKey, err := NewMasterKey(rawMasterKey)
	if err != nil {
		return nil, "", fmt.Errorf("NewMasterKey: %v\n", err)
	}
	return masterKey, derivedPass, nil
}

// v3

// EncryptionKey is used to encrypt and decrypt data
// these keys are used as the v3 KEK, DEK and v2/v3 file Keys
type EncryptionKey struct {
	Bytes  [32]byte
	Cipher cipher.AEAD
}

// MakeNewFileKey returns a new encryption key
func MakeNewFileKey(authVersion int) (*EncryptionKey, error) {
	switch authVersion {
	case 1, 2:
		encryptionKeyStr := GenerateRandomString(32)
		encryptionKey, err := MakeEncryptionKeyFromBytes([32]byte([]byte(encryptionKeyStr)))
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 2: %w", err)
		}
		return encryptionKey, nil
	default:
		encryptionKey, err := NewEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("NewKeyEncryptionKey auth version 3: %w", err)
		}
		return encryptionKey, nil
	}
}

// EncryptMeta should be avoided, and Filen.EncryptMeta should be used instead
func (key *EncryptionKey) EncryptMeta(metadata string) EncryptedString {
	nonce := [12]byte(GenerateRandomBytes(12))
	encrypted := key.Cipher.Seal(nil, nonce[:], []byte(metadata), nil)
	return NewEncryptedStringV3(encrypted, nonce)
}

// DecryptMeta should be avoided, and Filen.DecryptMeta should be used instead
func (key *EncryptionKey) DecryptMeta(metadata EncryptedString) (string, error) {
	if metadata[0:3] != "003" {
		return "", fmt.Errorf("unsupported metadata %s format (allowed: 003)", metadata[0:3])
	}
	nonce, err := hex.DecodeString(string(metadata[3:27]))
	if err != nil {
		return "", fmt.Errorf("decoding nonce: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(metadata[27:]))
	if err != nil {
		return "", fmt.Errorf("decoding metadata: %v", err)
	}
	decrypted, err := key.Cipher.Open(nil, nonce[:], decoded, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %v", err)
	}
	return string(decrypted), nil
}

// MakeEncryptionKeyFromBytes returns a new encryption key
// from a 32 byte array
func MakeEncryptionKeyFromBytes(key [32]byte) (*EncryptionKey, error) {
	c, err := getCipherForKey(key)
	if err != nil {
		return nil, fmt.Errorf("MakeEncryptionKeyFromBytes: %v", err)
	}
	return &EncryptionKey{
		Bytes:  key,
		Cipher: c,
	}, nil
}

// MakeEncryptionKeyFromStr returns a new encryption key
// from a 64 char hex encoded string
func MakeEncryptionKeyFromStr(key string) (*EncryptionKey, error) {
	decoded, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("decoding DEK: %w", err)
	}
	return MakeEncryptionKeyFromBytes([32]byte(decoded))
}

// NewEncryptionKey generates a new encryption key using a random 32 byte array
func NewEncryptionKey() (*EncryptionKey, error) {
	return MakeEncryptionKeyFromBytes([32]byte(GenerateRandomBytes(32)))
}

// ToString returns a 64 char hex encoded string representation
// of the encryption key
func (key *EncryptionKey) ToString() string {
	return hex.EncodeToString(key.Bytes[:])
}

// DeriveKEKAndAuthFromPassword returns a KEK and a DerivedPassword
// derived from the user password
func DeriveKEKAndAuthFromPassword(password string, salt string) (*EncryptionKey, DerivedPassword, error) {
	derived := hex.EncodeToString(argon2.IDKey([]byte(password), []byte(salt), 3, 65536, 4, 64))

	kek, err := MakeEncryptionKeyFromStr(derived[:len(derived)/2])
	if err != nil {
		return nil, "", fmt.Errorf("MakeEncryptionKeyFromBytes: %v", err)
	}
	return kek, DerivedPassword(derived[len(derived)/2:]), nil
}

// MakeEncryptionKeyFromUnknownStr returns a new encryption key
// from either a 32 character string or a 64 character hex encoded string
func MakeEncryptionKeyFromUnknownStr(key string) (*EncryptionKey, error) {
	switch len(key) {
	case 32: // v1 & v2
		return MakeEncryptionKeyFromBytes([32]byte([]byte(key)))
	case 64: // v3
		return MakeEncryptionKeyFromStr(key)
	default:
		return nil, fmt.Errorf("key length wrong")
	}
}

func (key *EncryptionKey) encrypt(nonce []byte, data []byte) []byte {
	return key.Cipher.Seal(data[:0], nonce, data, nil)
}

// EncryptData encrypts file data using the encryption key
// generates a nonce and prepends it to the data
func (key *EncryptionKey) EncryptData(data []byte) []byte {
	nonce := GenerateRandomBytes(12)
	data = key.encrypt(nonce[:], data)
	return append(nonce, data...)
}

func (key *EncryptionKey) decrypt(nonce []byte, data []byte) error {
	data, err := key.Cipher.Open(data[:0], nonce, data, nil)
	if err != nil {
		return fmt.Errorf("open: %v", err)
	}
	return nil
}

// DecryptData decrypts file data using the encryption key
// returns the decrypted data, assumes that the nonce is the first 12 bytes
func (key *EncryptionKey) DecryptData(data []byte) ([]byte, error) {
	nonce := data[:12]
	err := key.decrypt(nonce, data[12:])
	if err != nil {
		return nil, err
	}
	return data[12 : len(data)-key.Cipher.Overhead()], nil
}

func (key *EncryptionKey) ToStringWithAuthVersion(authVersion int) string {
	if authVersion == 3 {
		return hex.EncodeToString(key.Bytes[:])
	}
	return string(key.Bytes[:])
}

// RSAKeyPairFromStrings returns a private and public key pair
// from base64 encoded strings
func RSAKeyPairFromStrings(privKey string, pubKey string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	publicKeyDecoded, err := base64.StdEncoding.DecodeString(pubKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding public key: %v", err)
	}
	privateKeyDecoded, err := base64.StdEncoding.DecodeString(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding private key: %v", err)
	}
	publicKeyAny, err := x509.ParsePKIXPublicKey(publicKeyDecoded)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing public key: %v", err)
	}

	publicKey, ok := publicKeyAny.(*rsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("parsing public key, failed to cast: %v", err)
	}

	privateKeyAny, err := x509.ParsePKCS8PrivateKey(privateKeyDecoded)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing private key: %v", err)
	}

	privateKey, ok := privateKeyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("parsing private key, failed to cast: %v", err)
	}

	if !publicKey.Equal(&privateKey.PublicKey) {
		return nil, nil, fmt.Errorf("public and private key mismatch")
	}

	return privateKey, publicKey, nil
}

// HMACKey is a 256 bit key used as a generic hashing key
// any time we want a hash of a string
type HMACKey [32]byte

// MakeHMACKey derives a 256 bit key from a private key
// this is to allow a single key to derivable from both V2 and V3 accounts
func MakeHMACKey(privateKey *rsa.PrivateKey) HMACKey {
	key := HMACKey{}
	derivedKey := hkdf.New(sha256.New, privateKey.D.Bytes(), nil, []byte("hmac-sha256-key"))
	_, err := derivedKey.Read(key[:])
	if err != nil {
		// this should never happen
		// we do not read enough from the hkdf for it to be an issue
		panic("error generating hkdf key: " + err.Error())
	}
	return key
}

// Hash hashes a string using the key
func (h HMACKey) Hash(data []byte) string {
	hasher := hmac.New(sha256.New, h[:])
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}

// V2Hash hashes a string using the V2 algorithm
// this was used before HMACKey was introduced,
// and is still used in some places for v2 accounts
func V2Hash(data []byte) string {
	outerHasher := sha1.New()
	innerHasher := sha512.New()
	innerHasher.Write(data)
	outerHasher.Write([]byte(hex.EncodeToString(innerHasher.Sum(nil))))
	return hex.EncodeToString(outerHasher.Sum(nil))
}

// PublicEncrypt encrypts data using a public key
func PublicEncrypt(publicKey *rsa.PublicKey, data string) (EncryptedString, error) {
	encrypted, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, publicKey, []byte(data), nil)
	if err != nil {
		return "", err
	}
	return EncryptedString(base64.StdEncoding.EncodeToString(encrypted)), nil
}

// PublicKeyFromString returns a public key from a base64 encoded string
func PublicKeyFromString(pubKey string) (*rsa.PublicKey, error) {
	publicKeyDecoded, err := base64.StdEncoding.DecodeString(pubKey)
	if err != nil {
		return nil, fmt.Errorf("decoding public key: %v", err)
	}
	publicKeyAny, err := x509.ParsePKIXPublicKey(publicKeyDecoded)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %v", err)
	}
	publicKey, ok := publicKeyAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("parsing public key, failed to cast: %v", err)
	}
	return publicKey, nil
}
