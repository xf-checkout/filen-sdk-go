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

// fileUpload encapsulates the state of an ongoing file upload.
// It tracks the file being uploaded and maintains the upload context,
// encryption key, and hash calculation.
type fileUpload struct {
	types.IncompleteFile        // The file being uploaded
	uploadKey            string // Random key for this upload session
	ctx                  context.Context
	cancel               context.CancelCauseFunc
	hasher               hash.Hash // For calculating file hash during upload
}

// newFileUpload creates a new fileUpload structure from an IncompleteFile.
// It initializes a new random upload key and hash calculator for the upload process.
func (api *Filen) newFileUpload(ctx context.Context, cancel context.CancelCauseFunc, file *types.IncompleteFile) *fileUpload {
	return &fileUpload{
		IncompleteFile: *file,
		uploadKey:      crypto.GenerateRandomString(32),
		ctx:            ctx,
		cancel:         cancel,
		hasher:         sha512.New(),
	}
}

// uploadChunk encrypts and uploads a single chunk of a file.
// This function handles the encryption of the chunk before sending it to the server.
// It returns storage details (bucket and region) for the uploaded chunk.
func (api *Filen) uploadChunk(fu *fileUpload, chunkIndex int, data []byte) (*client.V3UploadResponse, error) {
	data = fu.EncryptionKey.EncryptData(data)
	response, err := api.Client.PostV3Upload(fu.ctx, fu.UUID, chunkIndex, fu.ParentUUID, fu.uploadKey, data)
	if err != nil {
		return nil, fmt.Errorf("upload chunk %d: %w", chunkIndex, err)
	}
	return response, nil
}

// makeEmptyRequestFromUploaderNoMeta creates a basic upload request without metadata.
// This is used as a foundation for both regular and empty file uploads.
func (api *Filen) makeEmptyRequestFromUploaderNoMeta(fu *fileUpload) *client.V3UploadEmptyRequest {
	return &client.V3UploadEmptyRequest{
		UUID:       fu.UUID,
		Name:       api.EncryptMeta(fu.Name),
		NameHashed: api.HashFileName(fu.Name),
		Size:       api.EncryptMeta("0"),
		Parent:     fu.ParentUUID,
		MimeType:   api.EncryptMeta(fu.MimeType),
		//Metadata: must be filled by caller
		Version: api.FileEncryptionVersion,
	}
}

// makeEmptyRequestFromUploader creates a complete upload request for an empty file.
// It includes encrypted metadata and file hash information.
func (api *Filen) makeEmptyRequestFromUploader(fu *fileUpload, fileHash string) (*client.V3UploadEmptyRequest, error) {
	metadata := fu.GetRawMeta(api.FileEncryptionVersion)
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

// makeRequestFromUploader creates a complete upload request for a non-empty file.
// It includes encrypted metadata, file size, chunk count, and hash information.
func (api *Filen) makeRequestFromUploader(fu *fileUpload, size int, fileHash string) (*client.V3UploadDoneRequest, error) {
	metadata := fu.GetRawMeta(api.FileEncryptionVersion)
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

// completeUpload finalizes the upload of a non-empty file.
// It sends the final metadata to the server and constructs the completed File object.
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
		Version:        api.FileEncryptionVersion,
	}, nil
}

// completeUploadEmpty finalizes the upload of an empty (zero-byte) file.
// It sends the appropriate metadata to the server and constructs the completed File object.
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

// UploadFile uploads a file to Filen cloud storage.
// It uses the provided IncompleteFile for metadata and reads file content from the io.Reader.
// The upload process happens in parallel chunks for efficiency.
// Once upload completes, it returns a File object representing the uploaded file.
//
// This method handles:
// - Chunking the file into manageable pieces
// - Parallel upload of chunks
// - Calculating file hash for integrity verification
// - Finalizing the upload with appropriate metadata
// - Updating search indexes and shared parent metadata
func (api *Filen) UploadFile(ctx context.Context, file *types.IncompleteFile, r io.Reader) (*types.File, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil) // Ensure context is canceled when we exit

	fileUpload := api.newFileUpload(ctx, cancel, file)
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
			case api.UploadThreadSem <- struct{}{}:
				wg.Add(1)
				go func(i int, chunk []byte) {
					defer func() {
						<-api.UploadThreadSem
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

	var (
		completeFile *types.File
		err          error
	)
	if size == 0 {
		completeFile, err = api.completeUploadEmpty(fileUpload)
		if err != nil {
			return nil, err
		}
	} else {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			select {
			case resp, ok := <-bucketAndRegion:
				if !ok {
					return nil, fmt.Errorf("no chunks successfully uploaded")
				}
				completeFile, err = api.completeUpload(fileUpload, resp.Bucket, resp.Region, size)
				if err != nil {
					return nil, fmt.Errorf("complete upload: %w", err)
				}
			case <-ctx.Done():
				return nil, fmt.Errorf("context done %w", context.Cause(ctx))
			}
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
