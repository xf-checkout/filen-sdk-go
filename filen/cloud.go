package filen

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/util"
	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"strings"
	"time"
)

// FindItem find a cloud item by its path and returns it (either the File or the Directory will be returned).
// Set requireDirectory to differentiate between files and directories with the same path (otherwise, the file will be found).
// Returns nil for both File and Directory if none was found.
func (api *Filen) FindItem(ctx context.Context, path string) (types.FileSystemObject, error) {

	var currentDir types.DirectoryInterface = &api.BaseFolder
	segments := strings.Split(path, "/")
	if len(strings.Join(segments, "")) == 0 {
		return currentDir, nil
	}

SegmentsLoop:
	for segmentIdx, segment := range segments {
		if segment == "" {
			continue
		}

		files, directories, err := api.ReadDirectory(ctx, currentDir)
		if err != nil {
			return nil, fmt.Errorf("read directory: %w", err)
		}
		for _, file := range files {
			if file.Name == segment {
				return file, nil
			}
		}
		for _, directory := range directories {
			if directory.Name == segment {
				if segmentIdx == len(segments)-1 {
					return directory, nil
				} else {
					currentDir = directory
					continue SegmentsLoop
				}
			}
		}
		return nil, nil
	}
	return nil, nil
}

func (api *Filen) FindFile(ctx context.Context, path string) (*types.File, error) {
	item, err := api.FindItem(ctx, path)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	file, ok := item.(*types.File)
	if !ok {
		return nil, fs.ErrorIsDir
	}
	return file, nil
}

func (api *Filen) FindDirectory(ctx context.Context, path string) (types.DirectoryInterface, error) {
	item, err := api.FindItem(ctx, path)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	directory, ok := item.(types.DirectoryInterface)
	if !ok {
		return nil, fs.ErrorIsFile
	}
	return directory, nil
}

// FindDirectoryOrCreate finds a cloud directory by its path and returns its UUID.
// If the directory cannot be found, it (and all non-existent parent directories) will be created.
func (api *Filen) FindDirectoryOrCreate(ctx context.Context, path string) (types.DirectoryInterface, error) {
	segments := strings.Split(path, "/")

	var currentDir types.DirectoryInterface = &api.BaseFolder
SegmentsLoop:
	for _, segment := range segments {
		if segment == "" {
			continue
		}

		_, directories, err := api.ReadDirectory(ctx, currentDir)
		if err != nil {
			return nil, err
		}
		for _, directory := range directories {
			if directory.Name == segment {
				// directory found
				currentDir = directory
				continue SegmentsLoop
			}
		}
		// create directory
		directory, err := api.CreateDirectory(ctx, currentDir, segment)
		if err != nil {
			return nil, err
		}
		currentDir = directory
	}
	return currentDir, nil
}

// ReadDirectory fetches the files and directories that are children of a directory (specified by UUID).
func (api *Filen) ReadDirectory(ctx context.Context, dir types.DirectoryInterface) ([]*types.File, []*types.Directory, error) {
	// fetch directory content
	directoryContent, err := api.Client.PostV3DirContent(ctx, dir.GetUUID())
	if err != nil {
		return nil, nil, fmt.Errorf("ReadDirectory fetching directory: %w", err)
	}

	// transform files
	files := make([]*types.File, 0)
	for _, file := range directoryContent.Uploads {
		metadataStr, err := api.DecryptMeta(file.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory decrypting metadata: %v", err)
		}
		var metadata types.FileMetadata
		err = json.Unmarshal([]byte(metadataStr), &metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory unmarshalling metadata: %v", err)
		}

		if len(metadata.Key) != 32 {

		}
		encryptionKey, err := crypto.MakeEncryptionKeyFromUnknownStr(metadata.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory creating encryption key: %v", err)
		}

		files = append(files, &types.File{
			IncompleteFile: types.IncompleteFile{
				UUID:          file.UUID,
				Name:          metadata.Name,
				MimeType:      metadata.MimeType,
				EncryptionKey: *encryptionKey,
				Created:       util.TimestampToTime(int64(metadata.Created)),
				LastModified:  util.TimestampToTime(int64(metadata.LastModified)),
				ParentUUID:    file.Parent,
			},
			Size:      metadata.Size,
			Favorited: file.Favorited == 1,
			Region:    file.Region,
			Bucket:    file.Bucket,
			Chunks:    file.Chunks,
			Hash:      metadata.Hash,
		})
	}

	// transform directories
	directories := make([]*types.Directory, 0)
	for _, directory := range directoryContent.Folders {
		metaStr, err := api.DecryptMeta(directory.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory decrypting metadata: %v", err)
		}
		metaData := types.DirectoryMetaData{}
		err = json.Unmarshal([]byte(metaStr), &metaData)
		if err != nil {
			return nil, nil, fmt.Errorf("ReadDirectory unmarshalling metadata: %v", err)
		}

		creationTimestamp := metaData.Creation
		if creationTimestamp == 0 {
			creationTimestamp = directory.Timestamp
		}

		directories = append(directories, &types.Directory{
			UUID:       directory.UUID,
			Name:       metaData.Name,
			ParentUUID: directory.Parent,
			Color:      directory.Color,
			Created:    util.TimestampToTime(int64(creationTimestamp)),
			Favorited:  directory.Favorited == 1,
		})
	}

	return files, directories, nil
}

func (api *Filen) ListRecursive(ctx context.Context, dir types.DirectoryInterface) ([]*types.File, []*types.Directory, error) {
	resp, err := api.Client.PostV3DirDownload(ctx, dir.GetUUID())
	if err != nil {
		return nil, nil, fmt.Errorf("ListRecursive fetching directory: %w", err)
	}
	files := make([]*types.File, 0, len(resp.Files))
	dirs := make([]*types.Directory, 0, len(resp.Folders))

	for _, file := range resp.Files {
		metaStr, err := api.DecryptMeta(file.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ListRecursive decrypting metadata: %v", err)
		}
		metadata := types.FileMetadata{}
		err = json.Unmarshal([]byte(metaStr), &metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ListRecursive unmarshalling metadata: %v", err)
		}

		encryptionKey, err := crypto.MakeEncryptionKeyFromUnknownStr(metadata.Key)
		if err != nil {
			return nil, nil, fmt.Errorf("ListRecursive creating encryption key: %v", err)
		}

		files = append(files, &types.File{
			IncompleteFile: types.IncompleteFile{
				UUID:          file.UUID,
				Name:          metadata.Name,
				MimeType:      metadata.MimeType,
				EncryptionKey: *encryptionKey,
				Created:       util.TimestampToTime(int64(metadata.Created)),
				LastModified:  util.TimestampToTime(int64(metadata.LastModified)),
				ParentUUID:    file.Parent,
			},
			Size:      metadata.Size,
			Favorited: false, // doesn't return favorited todo add tmrw when backend is updated
			Region:    file.Region,
			Bucket:    file.Bucket,
			Chunks:    file.Chunks,
			Hash:      metadata.Hash,
		})
	}

	for _, directory := range resp.Folders {
		if directory.Parent == "base" {
			// /v3/dir/download returns the dir it was called on as well with parent base
			continue
		}
		metaStr, err := api.DecryptMeta(directory.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("ListRecursive decrypting metadata: %v", err)
		}
		metaData := types.DirectoryMetaData{}
		err = json.Unmarshal([]byte(metaStr), &metaData)
		if err != nil {
			return nil, nil, fmt.Errorf("ListRecursive unmarshalling metadata: %v", err)
		}

		creationTimestamp := metaData.Creation
		if creationTimestamp == 0 {
			creationTimestamp = directory.Timestamp
		}

		dirs = append(dirs, &types.Directory{
			UUID:       directory.UUID,
			Name:       metaData.Name,
			ParentUUID: directory.Parent,
			Color:      "", // doesn't return colors todo add tmrw when backend is updated
			Created:    util.TimestampToTime(int64(creationTimestamp)),
			Favorited:  false, // doesn't return favorited value todo add tmrw when backend is updated
		})
	}
	return files, dirs, nil
}

// TrashFile moves a file to trash.
func (api *Filen) TrashFile(ctx context.Context, file types.File) error {
	return api.Client.PostV3FileTrash(ctx, file.GetUUID())
}

// CreateDirectory creates a new directory.
func (api *Filen) CreateDirectory(ctx context.Context, parent types.DirectoryInterface, name string) (*types.Directory, error) {
	if strings.ContainsRune(name, '/') {
		return nil, fmt.Errorf("invalid directory name")
	}
	directoryUUID := uuid.New().String()
	creationTime := time.Now().Round(time.Millisecond)
	// encrypt metadata
	metadata := types.DirectoryMetaData{
		Name:     name,
		Creation: int(creationTime.UnixMilli()),
	}
	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	metadataEncrypted := api.EncryptMeta(string(metadataStr))

	// hash name
	nameHashed := api.HashFileName(name)

	// send
	response, err := api.Client.PostV3DirCreate(ctx, directoryUUID, metadataEncrypted, nameHashed, parent.GetUUID())
	if err != nil {
		return nil, err
	}

	dir := &types.Directory{
		UUID:       response.UUID,
		Name:       name,
		ParentUUID: parent.GetUUID(),
		Color:      types.DirColorDefault,
		Created:    creationTime,
		Favorited:  false,
	}

	return dir, api.updateItemWithMaybeSharedParent(ctx, dir)
}

// TrashDirectory moves a directory to trash.
func (api *Filen) TrashDirectory(ctx context.Context, dir types.DirectoryInterface) error {
	return api.Client.PostV3DirTrash(ctx, dir.GetUUID())
}

func (api *Filen) FileExists(ctx context.Context, parentUUID string, name string) (*client.V3FileExistsResponse, error) {
	nameHashed := api.HashFileName(name)
	return api.Client.PostV3FileExists(ctx, nameHashed, parentUUID)
}

func (api *Filen) DirExists(ctx context.Context, parentUUID string, name string) (*client.V3DirExistsResponse, error) {
	nameHashed := api.HashFileName(name)
	return api.Client.PostV3DirExists(ctx, nameHashed, parentUUID)
}

func (api *Filen) MoveFile(ctx context.Context, file *types.File, newParentUUID string, overwrite bool) error {
	resp, err := api.FileExists(ctx, newParentUUID, file.GetName())
	if err != nil {
		return fmt.Errorf("FileExists: %w", err)
	}
	if resp.Exists {
		if overwrite {
			err = api.TrashFile(ctx, *file)
			if err != nil {
				return fmt.Errorf("TrashFile: %w", err)
			}
		} else {
			return fmt.Errorf("file already exists")
		}
	}

	err = api.Client.PostV3FileMove(ctx, file.GetUUID(), newParentUUID)
	if err != nil {
		return fmt.Errorf("PostV3FileMove: %w", err)
	}
	file.ParentUUID = newParentUUID
	return api.updateItemWithMaybeSharedParent(ctx, file)
}

func (api *Filen) MoveDir(ctx context.Context, dir *types.Directory, newParentUUID string, overwrite bool) error {
	resp, err := api.DirExists(ctx, newParentUUID, dir.GetName())
	if err != nil {
		return fmt.Errorf("DirExists: %w", err)
	}
	if resp.Exists {
		if overwrite {
			err = api.TrashDirectory(ctx, *dir)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("directory already exists")
		}
	}

	err = api.Client.PostV3DirMove(ctx, dir.GetUUID(), newParentUUID)
	if err != nil {
		return fmt.Errorf("PostV3DirMove: %w", err)
	}
	dir.ParentUUID = newParentUUID
	return api.updateItemWithMaybeSharedParent(ctx, dir)
}
