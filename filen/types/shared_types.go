package types

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/io"
	"github.com/google/uuid"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IncompleteFile struct {
	UUID          string // the UUID of the cloud item
	Name          string
	MimeType      string
	EncryptionKey crypto.EncryptionKey // the key used to encrypt the file data
	Created       time.Time            // when the file was created
	LastModified  time.Time            // when the file was last modified
	ParentUUID    string               // the [Directory.UUID] of the file's parent directory
}

func NewIncompleteFile(authVersion int, name string, mimeType string, created time.Time, lastModified time.Time, parent DirectoryInterface) (*IncompleteFile, error) {
	if strings.ContainsRune(name, '/') {
		return nil, fmt.Errorf("invalid file name")
	}
	key, err := crypto.MakeNewFileKey(authVersion)
	if err != nil {
		return nil, fmt.Errorf("make new file key: %w", err)
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	} else {
		mimeType, _, _ = strings.Cut(mimeType, ";")
	}

	return &IncompleteFile{
		UUID:          uuid.NewString(),
		Name:          name,
		MimeType:      mimeType,
		EncryptionKey: *key,
		Created:       created.Round(time.Millisecond),
		LastModified:  lastModified.Round(time.Millisecond),
		ParentUUID:    parent.GetUUID(),
	}, nil
}

func NewIncompleteFileFromOSFile(authVersion int, osFile *os.File, parent DirectoryInterface) (*IncompleteFile, error) {
	fileStat, err := osFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	created := io.GetCreationTime(fileStat)
	return NewIncompleteFile(authVersion, filepath.Base(osFile.Name()), "", created, fileStat.ModTime(), parent)
}

func (file IncompleteFile) NewFromBase(authVersion int) (*IncompleteFile, error) {
	key, err := crypto.MakeNewFileKey(authVersion)
	if err != nil {
		return nil, fmt.Errorf("make new file key: %w", err)
	}

	return &IncompleteFile{
		UUID:          uuid.NewString(),
		Name:          file.Name,
		MimeType:      file.MimeType,
		EncryptionKey: *key,
		Created:       file.Created,
		LastModified:  file.LastModified,
		ParentUUID:    file.ParentUUID,
	}, nil
}

func (file IncompleteFile) GetRawMeta(authVersion int) FileMetadata {
	return FileMetadata{
		Name:         file.Name,
		Size:         0,
		MimeType:     file.MimeType,
		Key:          file.EncryptionKey.ToStringWithAuthVersion(authVersion),
		LastModified: int(file.LastModified.UnixMilli()),
		Created:      int(file.Created.UnixMilli()),
		Hash:         "",
	}
}

type FileMetadata struct {
	Name         string `json:"name"`
	Size         int    `json:"size"`
	MimeType     string `json:"mime"`
	Key          string `json:"key"`
	LastModified int    `json:"lastModified"`
	Created      int    `json:"creation"`
	Hash         string `json:"hash"`
}

// File represents a file on the cloud drive.
type File struct {
	IncompleteFile
	Size      int    // the file size in bytes
	Favorited bool   // whether the file is marked a favorite
	Region    string // the file's storage region
	Bucket    string // the file's storage bucket
	Chunks    int    // how many 1 MiB chunks the file is partitioned into
	Hash      string // the file's SHA512 hash
}

type DirColor string

const (
	DirColorDefault DirColor = ""
	DirColorBlue    DirColor = "blue"
	DirColorGreen   DirColor = "green"
	DirColorPurple  DirColor = "purple"
	DirColorRed     DirColor = "red"
	DirColorGray    DirColor = "gray"
)

type DirectoryMetaData struct {
	Name     string `json:"name"`
	Creation int    `json:"creation"`
}

// Directory represents a directory on the cloud drive.
type Directory struct {
	UUID       string    // the UUID of the cloud item
	Name       string    // the directory name
	ParentUUID string    // the [Directory.UUID] of the directory's parent directory (or zero value for the root directory)
	Color      DirColor  // the color assigned to the directory (zero value means default color)
	Created    time.Time // when the directory was created
	Favorited  bool      // whether the directory is marked a favorite
}

type RootDirectory struct {
	UUID string
}

func NewRootDirectory(uuid string) RootDirectory {
	return RootDirectory{UUID: uuid}
}

type FileSystemObject interface {
	GetUUID() string
	GetName() string
	GetParent() string
}

type NonRootFileSystemObject interface {
	FileSystemObject
	GetMeta(authVersion int) (string, error)
}

type DirectoryInterface interface {
	FileSystemObject
	IsRoot() bool
}

func (file File) GetUUID() string {
	return file.IncompleteFile.UUID
}

func (file File) GetName() string {
	return file.Name
}

func (file File) GetParent() string {
	return file.ParentUUID
}

func (file File) GetMeta(authVersion int) (string, error) {
	meta := file.GetRawMeta(authVersion)
	meta.Size = file.Size
	meta.Hash = file.Hash
	metaStr, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal file metadata: %w", err)
	}
	return string(metaStr), nil
}

func (dir Directory) GetUUID() string {
	return dir.UUID
}

func (dir Directory) GetName() string {
	return dir.Name
}

func (dir Directory) GetParent() string {
	return dir.ParentUUID
}

func (dir Directory) IsRoot() bool {
	return false
}

func (dir Directory) GetMeta(authVersion int) (string, error) {
	meta := DirectoryMetaData{
		Name:     dir.Name,
		Creation: int(dir.Created.Unix()),
	}
	metaStr, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("marshal directory metadata: %w", err)
	}
	return string(metaStr), nil
}

func (root RootDirectory) GetUUID() string {
	return root.UUID
}

func (root RootDirectory) GetName() string {
	return ""
}

func (root RootDirectory) GetParent() string {
	return ""
}

func (root RootDirectory) IsRoot() bool {
	return true
}

type CtxMutex struct {
	channel chan struct{}
}

func NewCtxMutex() CtxMutex {
	return CtxMutex{
		channel: make(chan struct{}, 1),
	}
}

func (m *CtxMutex) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case m.channel <- struct{}{}:
		return nil
	}
}

func (m *CtxMutex) BlockUntilLock() {
	select {
	case m.channel <- struct{}{}:
		return
	}
}

func (m *CtxMutex) MustLock() {
	select {
	case m.channel <- struct{}{}:
		return
	default:
		panic("locking locked mutex")
	}
}

func (m *CtxMutex) Unlock() {
	select {
	case <-m.channel:
		return
	default:
		panic("unlocking unlocked mutex")
	}
}
