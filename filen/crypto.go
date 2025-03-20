package filen

import (
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"strings"
)

// HashFileName hashes a file name, this is used for file and directory names
// and is dependent on the auth version, version 1 and 2 use the crypto.V2Hash
// function, version 3 uses the HMACKey
func (api *Filen) HashFileName(name string) string {
	name = strings.ToLower(name)
	switch api.AuthVersion {
	case 1, 2:
		return crypto.V2Hash([]byte(name))
	default:
		return api.HMACKey.Hash([]byte(name))
	}
}

// EncryptMeta encrypts metadata, this is dependent on the auth version
// version 1 is unimplemented, 2 uses the MasterKeys, and 3 uses the DEK
func (api *Filen) EncryptMeta(metadata string) crypto.EncryptedString {
	switch api.AuthVersion {
	case 1:
		panic("todo")
	case 2:
		return api.MasterKeys.EncryptMeta(metadata)
	case 3:
		return api.DEK.EncryptMeta(metadata)
	default:
		panic("unsupported version")
	}
}

// DecryptMeta decrypts metadata, this reads the encrypted string to determine
// the version, and then uses the MasterKeys or DEK
func (api *Filen) DecryptMeta(encrypted crypto.EncryptedString) (string, error) {
	if encrypted[0:8] == "U2FsdGVk" {
		return api.MasterKeys.DecryptMetaV1(encrypted)
	}
	switch encrypted[0:3] {
	case "002":
		return api.MasterKeys.DecryptMetaV2(encrypted)
	case "003":
		return api.DEK.DecryptMeta(encrypted)
	default:
		panic("unsupported version")
	}
}
