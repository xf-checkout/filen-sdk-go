package filen

import "github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"

const (
	// ChunkSize is the maximum size of a chunk in bytes
	// as defined by Filen when uploading files (1 MiB)
	ChunkSize = 1024 * 1024
	// MaxUploaders is the maximum number of concurrent upload goroutines
	// running at the same time, this is mostly arbitrary
	// but 16 should easily be able to manage 10MiB/s of uploads
	MaxUploaders = 16
	// MaxSmallCallers is the maximum number of concurrent goroutines
	// running at the same time making smaller requests, this is mostly arbitrary
	// and mainly used to limit the number of API calls during a large DirMove from rclone
	MaxSmallCallers = 64
	// MaxDownloadBufferSize is the number of chunks to keep in memory at once.
	// This controls memory usage during downloads.
	MaxDownloadBufferSize              = 8
	V2AccountFileEncryptionVersion     = crypto.FileEncryptionVersion(2)
	V2AccountMetadataEncryptionVersion = crypto.MetadataEncryptionVersion(2)
)
