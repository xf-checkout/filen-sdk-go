package filen

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"hash"
	"io"
	"sync"
)

// fetchAndDecryptChunk downloads and decrypts a single chunk of a file.
// It retrieves the encrypted chunk from the Filen servers and decrypts it
// using the file's encryption key.
func (api *Filen) fetchAndDecryptChunk(ctx context.Context, file *types.File, chunkIndex int) ([]byte, error) {
	// could potentially be optimized by accepting a []byte buffer to reuse
	encryptedBytes, err := api.Client.DownloadFileChunk(ctx, file.UUID, file.Region, file.Bucket, chunkIndex)
	if err != nil {
		return nil, fmt.Errorf("downloading chunk %d: %w", chunkIndex, err)
	}
	var decryptedBytes []byte
	if file.Version == 1 {
		decryptedBytes, err = crypto.V1Decrypt(encryptedBytes, file.EncryptionKey.Bytes[:])
	} else {
		decryptedBytes, err = file.EncryptionKey.DecryptData(encryptedBytes)
	}
	if err != nil {
		return nil, fmt.Errorf("decrypting chunk %d: %w", chunkIndex, err)
	}
	return decryptedBytes, nil
}

// chunkState represents the state of a single chunk in the buffer.
// It handles concurrent access to chunk data and tracks the actual size
// of the chunk data (which may be less than ChunkSize for the last chunk).
type chunkState struct {
	data  [ChunkSize]byte // Fixed-size array for optimal cache locality
	size  int             // Actual size of data (may be less than ChunkSize for the last chunk)
	ctxMu types.CtxMutex  // Mutex for this specific chunk
}

// copyTo copies data from the chunk to the provided output buffer,
// starting at the specified offset and copying up to maxLength bytes.
// It respects the context for cancellation and properly synchronizes access.
func (c *chunkState) copyTo(ctx context.Context, out []byte, offset int, maxLength int) (int, error) {
	err := c.ctxMu.Lock(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to lock CtxMutex: %w", err)
	}
	defer c.ctxMu.Unlock()
	if offset >= c.size {
		return 0, io.EOF
	}

	available := c.size - offset
	if maxLength > available {
		maxLength = available
	}

	copy(out, c.data[offset:offset+maxLength])
	return maxLength, nil
}

// ChunkedReader implements io.Reader for sequential chunked file downloads.
// It provides efficient streaming access to files stored in Filen cloud storage
// by downloading chunks in parallel and validating file integrity.
type ChunkedReader struct {
	file              *types.File             // The file being downloaded
	api               *Filen                  // API client to use for downloading
	buffer            []chunkState            // Fixed-size circular buffer of chunks
	chunkIndex        int                     // Index of the current chunk being read
	offsetInChunk     int                     // Current offset within the current chunk
	ctx               context.Context         // Context for cancellation
	cancel            context.CancelCauseFunc // Function to cancel with cause
	errOnce           *sync.Once              // Ensures error is reported only once
	hasher            hash.Hash               // For calculating file hash during download
	lastChunkIndex    int                     // Index of the last chunk to read
	lastOffsetInChunk int                     // Last valid offset in the final chunk
	totalRead         int                     // Total bytes read, -1 if started with an offset
}

// newChunkedReaderWithOffset creates a new ChunkedReader for sequential reading,
// starting at the specified byte offset and reading up to the specified limit.
// If limit is -1, reads to the end of the file.
func newChunkedReaderWithOffset(ctx context.Context, api *Filen, file *types.File, offset int, limit int) *ChunkedReader {
	if limit == -1 {
		limit = file.Size
	} else {
		limit = min(limit, file.Size)
	}

	chunkIndex := 0
	offsetInChunk := 0
	totalRead := 0
	if offset > 0 {
		chunkIndex = offset / ChunkSize
		offsetInChunk = offset % ChunkSize
		totalRead = -1
	}
	lastChunkIndex := min(file.Chunks-1, limit/ChunkSize)
	lastOffsetInChunk := file.Size % ChunkSize
	if limit != -1 {
		lastOffsetInChunk = limit % ChunkSize
	}

	if lastOffsetInChunk == 0 && lastChunkIndex == file.Chunks-1 {
		lastOffsetInChunk = ChunkSize
	}

	ctx, cancel := context.WithCancelCause(ctx)
	bufferSize := min(MaxDownloadBufferSize, lastChunkIndex-chunkIndex+1)

	reader := &ChunkedReader{
		file:              file,
		api:               api,
		buffer:            make([]chunkState, bufferSize),
		chunkIndex:        chunkIndex,
		offsetInChunk:     offsetInChunk,
		ctx:               ctx,
		cancel:            cancel,
		errOnce:           &sync.Once{},
		hasher:            sha512.New(),
		lastChunkIndex:    lastChunkIndex,
		lastOffsetInChunk: lastOffsetInChunk,
		totalRead:         totalRead,
	}

	// Init and prefetch initial chunks
	for i := 0; i < bufferSize; i++ {
		reader.buffer[(i+chunkIndex)%bufferSize].ctxMu = types.NewCtxMutex()
		reader.goFetchChunk(i + chunkIndex)
	}
	return reader
}

// newChunkedReader creates a new ChunkedReader that reads the entire file
// from the beginning.
func newChunkedReader(ctx context.Context, api *Filen, file *types.File) *ChunkedReader {
	return newChunkedReaderWithOffset(ctx, api, file, 0, -1)
}

// fetchChunk downloads and decrypts a specific chunk, storing it in the provided
// chunkState. If an error occurs, it cancels the reader context with the error.
func (r *ChunkedReader) fetchChunk(c *chunkState, chunkIndex int) {
	data, err := r.api.fetchAndDecryptChunk(r.ctx, r.file, chunkIndex)
	if err != nil {
		r.errOnce.Do(func() { r.cancel(fmt.Errorf("failed to fetch chunk %d: %w", chunkIndex, err)) })
		return
	}
	if len(data) > ChunkSize {
		r.errOnce.Do(func() { r.cancel(fmt.Errorf("chunk %d is too large: %d bytes", chunkIndex, len(data))) })
		return
	}
	copy(c.data[:], data)
	c.size = len(data)
}

// goFetchChunk asynchronously fetches a chunk in the background.
// It ensures the chunk is within bounds and properly acquires the mutex
// for the chunk's buffer slot before starting the fetch operation.
func (r *ChunkedReader) goFetchChunk(chunkIndex int) {
	if chunkIndex > r.lastChunkIndex {
		return
	}
	bufferPos := chunkIndex % len(r.buffer)
	chunkState := &r.buffer[bufferPos]
	chunkState.ctxMu.MustLock()
	go func() {
		defer chunkState.ctxMu.Unlock()
		r.fetchChunk(chunkState, chunkIndex)
	}()
}

// Read implements the io.Reader interface, optimized for sequential reading.
// It reads data from the file in chunks, handling prefetching of future chunks
// and validating the file hash as data is read.
func (r *ChunkedReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Check for fetch errors
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	read := 0
	for read < len(p) {
		select {
		case <-r.ctx.Done():
			return read, r.ctx.Err()
		default:
			// continue
		}
		// Check if we've reached EOF
		if r.chunkIndex > r.lastChunkIndex {
			if read == 0 {
				return 0, io.EOF
			}
			break
		}

		toRead := len(p) - read
		if r.chunkIndex == r.lastChunkIndex {
			toRead = min(toRead, r.lastOffsetInChunk-r.offsetInChunk)
		}

		currentChunk := &r.buffer[r.chunkIndex%len(r.buffer)]
		copiedLen, err := currentChunk.copyTo(r.ctx, p[read:], r.offsetInChunk, toRead)

		if r.totalRead != -1 {
			r.totalRead += copiedLen
		}

		if err == io.EOF {
			// this shouldn't really happen, but just in case
			r.goFetchChunk(r.chunkIndex + len(r.buffer))
			r.offsetInChunk = 0
			r.chunkIndex++
			continue
		} else if err != nil {
			return read, fmt.Errorf("failed to read chunk: %w", err)
		}
		read += copiedLen
		r.offsetInChunk += copiedLen
		// Check if finished reading chunk
		if r.offsetInChunk >= currentChunk.size || (r.chunkIndex >= r.lastChunkIndex && r.offsetInChunk >= r.lastOffsetInChunk) {
			r.goFetchChunk(r.chunkIndex + len(r.buffer))
			r.offsetInChunk = 0
			r.chunkIndex++
		}
	}
	r.hasher.Write(p[:read])
	return read, nil
}

// Close cleans up resources used by the reader and verifies the file hash
// if the entire file was read. It should be called when done with the reader
// to ensure proper cleanup and validation.
func (r *ChunkedReader) Close() error {
	r.cancel(fmt.Errorf("reader closed")) // Cancel all ongoing operations

	if r.totalRead != r.file.Size {
		// incomplete read
		return nil
	}
	if r.file.Hash != "" {
		h := hex.EncodeToString(r.hasher.Sum(nil))
		if r.file.Hash != h {
			return fmt.Errorf("hash mismatch: expected %s, got %s", r.file.Hash, h)
		}
	}
	// should we be replacing the hash if it's empty?
	return nil
}
