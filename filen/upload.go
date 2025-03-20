package filen

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"hash"
	"io"
	"strconv"
	"sync"

	"github.com/FilenCloudDienste/filen-sdk-go/filen/client"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
)

type fileUpload struct {
	types.IncompleteFile
	uploadKey string
	ctx       context.Context
	cancel    context.CancelCauseFunc
	hasher    hash.Hash
}

func (api *Filen) newFileUpload(ctx context.Context, cancel context.CancelCauseFunc, file *types.IncompleteFile) *fileUpload {
	return &fileUpload{
		IncompleteFile: *file,
		uploadKey:      crypto.GenerateRandomString(32),
		ctx:            ctx,
		cancel:         cancel,
		hasher:         sha512.New(),
	}
}

func (api *Filen) uploadChunk(fu *fileUpload, chunkIndex int, data []byte) (*client.V3UploadResponse, error) {
	data = fu.EncryptionKey.EncryptData(data)
	response, err := api.Client.PostV3Upload(fu.ctx, fu.UUID, chunkIndex, fu.ParentUUID, fu.uploadKey, data)
	if err != nil {
		return nil, fmt.Errorf("upload chunk %d: %w", chunkIndex, err)
	}
	return response, nil
}

func (api *Filen) makeEmptyRequestFromUploaderNoMeta(fu *fileUpload) *client.V3UploadEmptyRequest {
	return &client.V3UploadEmptyRequest{
		UUID:       fu.UUID,
		Name:       api.EncryptMeta(fu.Name),
		NameHashed: api.HashFileName(fu.Name),
		Size:       api.EncryptMeta("0"),
		Parent:     fu.ParentUUID,
		MimeType:   api.EncryptMeta(fu.MimeType),
		//Metadata: must be filled by caller
		Version: api.AuthVersion,
	}
}

func (api *Filen) makeEmptyRequestFromUploader(fu *fileUpload, fileHash string) (*client.V3UploadEmptyRequest, error) {
	metadata := fu.GetRawMeta(api.AuthVersion)
	metadata.Size = 0
	metadata.Hash = fileHash

	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}
	emptyRequest := api.makeEmptyRequestFromUploaderNoMeta(fu)
	emptyRequest.Metadata = api.EncryptMeta(string(metadataStr))

	return emptyRequest, nil
}

func (api *Filen) makeRequestFromUploader(fu *fileUpload, size int, fileHash string) (*client.V3UploadDoneRequest, error) {
	metadata := fu.GetRawMeta(api.AuthVersion)
	metadata.Size = size
	metadata.Hash = fileHash

	metadataStr, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal file metadata: %w", err)
	}
	emptyRequest := api.makeEmptyRequestFromUploaderNoMeta(fu)
	emptyRequest.Metadata = api.EncryptMeta(string(metadataStr))
	emptyRequest.Size = api.EncryptMeta(strconv.Itoa(size))

	return &client.V3UploadDoneRequest{
		V3UploadEmptyRequest: *emptyRequest,
		Chunks:               (size + ChunkSize - 1) / ChunkSize,
		UploadKey:            fu.uploadKey,
		Rm:                   crypto.GenerateRandomString(32),
	}, nil
}

func (api *Filen) completeUpload(fu *fileUpload, bucket string, region string, size int) (*types.File, error) {
	fileHash := hex.EncodeToString(fu.hasher.Sum(nil))
	uploadRequest, err := api.makeRequestFromUploader(fu, size, fileHash)
	if err != nil {
		return nil, fmt.Errorf("make request from uploader: %w", err)
	}
	_, err = api.Client.PostV3UploadDone(fu.ctx, *uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}

	return &types.File{
		IncompleteFile: fu.IncompleteFile,
		Size:           size,
		Region:         region,
		Bucket:         bucket,
		Chunks:         uploadRequest.Chunks,
		Hash:           fileHash,
	}, nil
}

func (api *Filen) completeUploadEmpty(fu *fileUpload) (*types.File, error) {
	fileHash := hex.EncodeToString(fu.hasher.Sum(nil))
	uploadRequest, err := api.makeEmptyRequestFromUploader(fu, fileHash)
	if err != nil {
		return nil, fmt.Errorf("make request from uploader: %w", err)
	}
	_, err = api.Client.PostV3UploadEmpty(fu.ctx, *uploadRequest)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}

	return &types.File{
		IncompleteFile: fu.IncompleteFile,
		Size:           0,
		Region:         "",
		Bucket:         "",
		Chunks:         0,
		Hash:           fileHash,
	}, nil

}

func (api *Filen) UploadFile(ctx context.Context, file *types.IncompleteFile, r io.Reader) (*types.File, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil) // Ensure context is canceled when we exit

	fileUpload := api.newFileUpload(ctx, cancel, file)
	uploadSem := make(chan struct{}, MaxUploaders)
	wg := sync.WaitGroup{}
	bucketAndRegion := make(chan client.V3UploadResponse, 1)
	size := 0

	for i := 0; ; i++ {
		data := make([]byte, ChunkSize, ChunkSize+file.EncryptionKey.Cipher.Overhead())
		read, err := r.Read(data)
		size += read

		if err != nil && err != io.EOF {
			fileUpload.cancel(fmt.Errorf("read chunk %d: %w", i, err))
			return nil, err
		}

		if read > 0 {
			if read < ChunkSize {
				data = data[:read]
			}
			fileUpload.hasher.Write(data)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context done %w", context.Cause(ctx))
			case uploadSem <- struct{}{}:
				wg.Add(1)
				go func(i int, chunk []byte) {
					defer func() {
						<-uploadSem
						wg.Done()
					}()

					resp, err := api.uploadChunk(fileUpload, i, data)
					if err != nil {
						cancel(err)
						return
					}
					select { // only care about getting this once
					case bucketAndRegion <- *resp:
					default:
					}
				}(i, data)
			}
		}

		if err == io.EOF {
			break
		}
	}

	if size == 0 {
		return api.completeUploadEmpty(fileUpload)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	var completeFile *types.File
	select {
	case <-done:
		select {
		case resp, ok := <-bucketAndRegion:
			if !ok {
				return nil, fmt.Errorf("no chunks successfully uploaded")
			}
			var err error
			completeFile, err = api.completeUpload(fileUpload, resp.Bucket, resp.Region, size)
			if err != nil {
				return nil, fmt.Errorf("complete upload: %w", err)
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("context done %w", context.Cause(ctx))
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return api.updateItemWithMaybeSharedParent(gCtx, completeFile) })
	g.Go(func() error { return api.updateSearchHashes(gCtx, completeFile) })
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return completeFile, nil
}
