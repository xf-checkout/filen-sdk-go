package filen

import (
	"context"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"io"
)

type SerializableFilen struct {
	APIKey         string
	AuthVersion    int
	Email          string
	MasterKeys     [][64]byte
	DEK            [32]byte
	KEK            [32]byte
	PrivateKey     []byte
	HMACKey        [32]byte
	BaseFolderUUID string
}

func (api *Filen) serialize() *SerializableFilen {
	masterKeys := make([][64]byte, len(api.MasterKeys))
	for i, masterKey := range api.MasterKeys {
		masterKeys[i] = masterKey.Bytes
	}
	return &SerializableFilen{
		APIKey:         api.Client.APIKey,
		AuthVersion:    api.AuthVersion,
		Email:          api.Email,
		MasterKeys:     masterKeys,
		DEK:            api.DEK.Bytes,
		PrivateKey:     x509.MarshalPKCS1PrivateKey(&api.PrivateKey),
		HMACKey:        api.HMACKey,
		BaseFolderUUID: api.BaseFolder.GetUUID(),
	}
}

func (s *SerializableFilen) deserialize() (*Filen, error) {
	masterKeys := make([]crypto.MasterKey, len(s.MasterKeys))
	for i, masterKey := range s.MasterKeys {
		masterKey, err := crypto.NewMasterKey(masterKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse master key: %w", err)
		}
		masterKeys[i] = *masterKey
	}
	var (
		dek crypto.EncryptionKey
	)
	if s.AuthVersion >= 3 {
		dekPtr, err := crypto.MakeEncryptionKeyFromBytes(s.DEK)
		if err != nil {
			return nil, fmt.Errorf("failed to parse DEK: %w", err)
		}
		dek = *dekPtr
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(s.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Filen{
		Client:      client.NewWithAPIKey(context.Background(), s.APIKey),
		AuthVersion: s.AuthVersion,
		Email:       s.Email,
		MasterKeys:  masterKeys,
		DEK:         dek,
		PrivateKey:  *privateKey,
		PublicKey:   privateKey.PublicKey,
		HMACKey:     s.HMACKey,
		BaseFolder:  types.NewRootDirectory(s.BaseFolderUUID),
	}, nil
}

func (api *Filen) SerializeTo(w io.Writer) error {
	s := api.serialize()
	encoder := gob.NewEncoder(w)
	return encoder.Encode(s)
}

func DeserializeFrom(r io.Reader) (*Filen, error) {
	var s SerializableFilen
	decoder := gob.NewDecoder(r)
	if err := decoder.Decode(&s); err != nil {
		return nil, err
	}
	return s.deserialize()
}

type TSConfig struct {
	Email          string
	MasterKeys     []string
	APIKey         string
	PublicKey      string
	PrivateKey     string
	AuthVersion    int
	BaseFolderUUID string
}

func NewFromTSConfig(tsconfig TSConfig) (*Filen, error) {
	switch tsconfig.AuthVersion {
	case 2:
		masterKeys := make([]crypto.MasterKey, len(tsconfig.MasterKeys))
		for i, masterKey := range tsconfig.MasterKeys {
			if len(masterKey) != 64 {
				return nil, fmt.Errorf("invalid master key length: %d", len(masterKey))
			}
			masterKey, err := crypto.NewMasterKey([64]byte([]byte(masterKey)))
			if err != nil {
				panic(err)
			}
			masterKeys[i] = *masterKey
		}
		privateKey, publicKey, err := crypto.RSAKeyPairFromStrings(tsconfig.PrivateKey, tsconfig.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rsa keys: %w", err)
		}
		return &Filen{
			Client:      client.NewWithAPIKey(context.Background(), tsconfig.APIKey),
			AuthVersion: tsconfig.AuthVersion,
			Email:       tsconfig.Email,
			MasterKeys:  masterKeys,
			PrivateKey:  *privateKey,
			PublicKey:   *publicKey,
			BaseFolder:  types.NewRootDirectory(tsconfig.BaseFolderUUID),
		}, nil
	case 3:
		dek, err := crypto.MakeEncryptionKeyFromStr(tsconfig.MasterKeys[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse DEK: %w", err)
		}
		private, public, err := crypto.RSAKeyPairFromStrings(tsconfig.PrivateKey, tsconfig.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse rsa keys: %w", err)
		}
		return &Filen{
			Client:      client.NewWithAPIKey(context.Background(), tsconfig.APIKey),
			AuthVersion: tsconfig.AuthVersion,
			Email:       tsconfig.Email,
			MasterKeys:  make(crypto.MasterKeys, 0),
			DEK:         *dek,
			PrivateKey:  *private,
			PublicKey:   *public,
			HMACKey:     crypto.MakeHMACKey(private),
			BaseFolder:  types.NewRootDirectory(tsconfig.BaseFolderUUID),
		}, nil
	default:
		return nil, fmt.Errorf("invalid auth version: %d", tsconfig.AuthVersion)
	}
}
