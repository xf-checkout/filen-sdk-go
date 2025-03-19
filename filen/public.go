package filen

import (
	"context"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"io"
	"os"
	"path"
)

// DownloadToPath downloads a file from the cloud to the given downloadPath.
// The file is first downloaded to a temporary file in the same directory,
// then renamed to the final path. If an error occurs during download or rename,
// the temporary file is removed.
func (api *Filen) DownloadToPath(ctx context.Context, file *types.File, downloadPath string) error {
	downloadDir := path.Dir(downloadPath)
	// needs to be removed or renamed
	f, err := os.CreateTemp(downloadDir, fmt.Sprintf("%s-download-*.tmp", file.Name))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	fName := f.Name()
	downloader := api.GetDownloadReader(ctx, file)
	_, err = f.ReadFrom(downloader)
	errClose := f.Close()
	if err != nil {
		_ = os.Remove(fName)
		maybeErr := context.Cause(ctx)
		if maybeErr != nil {
			return fmt.Errorf("download file: %w", maybeErr)
		}
		return fmt.Errorf("download file: %w", err)
	}

	err = downloader.Close()
	if err != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("close downloader: %w", err)
	}

	if errClose != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("close file: %w", errClose)
	}
	// should be okay because the temp file is in the same directory
	err = os.Rename(f.Name(), downloadPath)
	if err != nil {
		_ = os.Remove(fName)
		return fmt.Errorf("rename file: %w", err)
	}
	return nil
}

func (api *Filen) GetDownloadReader(ctx context.Context, file *types.File) io.ReadCloser {
	return newChunkedReader(ctx, api, file)
}

func (api *Filen) GetDownloadReaderWithOffset(ctx context.Context, file *types.File, offset int, limit int) io.ReadCloser {
	return newChunkedReaderWithOffset(ctx, api, file, offset, limit)
}

func (api *Filen) UploadFromReader(ctx context.Context, file *types.IncompleteFile, r io.Reader) (*types.File, error) {
	return api.UploadFile(ctx, file, r)
}

func (api *Filen) updateFileMeta(ctx context.Context, file *types.File, metaEncrypted crypto.EncryptedString, nameHashed string) error {
	nameEncrypted := file.EncryptionKey.EncryptMeta(file.Name)
	return api.Client.PostV3FileMetadata(ctx, file.UUID, nameEncrypted, nameHashed, metaEncrypted)
}

func (api *Filen) updateDirMeta(ctx context.Context, dir *types.Directory, metaEncrypted crypto.EncryptedString, nameHashed string) error {
	return api.Client.PostV3DirMetadata(ctx, dir.UUID, nameHashed, metaEncrypted)
}

func (api *Filen) UpdateMeta(ctx context.Context, item types.NonRootFileSystemObject) error {
	metaStr, err := item.GetMeta(api.AuthVersion)
	if err != nil {
		return fmt.Errorf("get meta: %w", err)
	}
	metaEncrypted := api.EncryptMeta(metaStr)

	nameHashed := api.HashFileName(item.GetName())

	if dir, ok := item.(*types.Directory); ok {
		err = api.updateDirMeta(ctx, dir, metaEncrypted, nameHashed)
	} else if file, ok := item.(*types.File); ok {
		err = api.updateFileMeta(ctx, file, metaEncrypted, nameHashed)
	} else {
		return fmt.Errorf("unknown item type")
	}

	return api.updateItemWithMaybeSharedParent(ctx, item)
}

func (api *Filen) Rename(ctx context.Context, item types.NonRootFileSystemObject, newName string) error {
	oldName := item.GetName()
	if dir, ok := item.(*types.Directory); ok {
		dir.Name = newName
		err := api.UpdateMeta(ctx, item)
		if err != nil {
			dir.Name = oldName
			return fmt.Errorf("update meta: %w", err)
		}
	} else if file, ok := item.(*types.File); ok {
		file.Name = newName
		err := api.UpdateMeta(ctx, item)
		if err != nil {
			file.Name = oldName
			return fmt.Errorf("update meta: %w", err)
		}
	} else {
		return fmt.Errorf("unknown item type")
	}
	return nil
}
