package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3DirDownloadRequest struct {
	UUID      string `json:"uuid"`
	SkipCache string `json:"skipCache"`
}

type v3DirDownloadLinkedRequest struct {
	UUID       string `json:"uuid"`
	SkipCache  string `json:"skipCache"`
	ParentUUID string `json:"parent"`
	Password   string `json:"password"`
}

type V3DirDownloadResponse struct {
	Files []struct {
		UUID     string                 `json:"uuid"`
		Bucket   string                 `json:"bucket"`
		Region   string                 `json:"region"`
		Chunks   int                    `json:"chunks"`
		Parent   string                 `json:"parent"`
		Metadata crypto.EncryptedString `json:"metadata"`
		Version  int                    `json:"version"`

		// optional
		Name       string `json:"name"`
		Size       string `json:"size"`
		MimeType   string `json:"mime"`
		ChunksSize int    `json:"chunksSize"` // no idea what this is
		Timestamp  int    `json:"timestamp"`
		Favorited  bool   `json:"favorited"`
	} `json:"files"`
	Folders []struct {
		UUID     string                 `json:"uuid"`
		Metadata crypto.EncryptedString `json:"name"` // name is actually the metadata
		Parent   string                 `json:"parent"`

		// optional
		Timestamp int    `json:"timestamp"`
		Color     string `json:"color"`
		Favorited bool   `json:"favorited"`
	} `json:"folders"`
}

func (c *Client) postV3DirDownload(ctx context.Context, endpoint string, req v3DirDownloadRequest) (*V3DirDownloadResponse, error) {
	var resp V3DirDownloadResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL(endpoint), req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) PostV3DirDownload(ctx context.Context, uuid string) (*V3DirDownloadResponse, error) {
	return c.postV3DirDownload(ctx, "/v3/dir/download", v3DirDownloadRequest{
		UUID: uuid,
	})
}

func (c *Client) PostV3DirDownloadShared(ctx context.Context, uuid string) (*V3DirDownloadResponse, error) {
	return c.postV3DirDownload(ctx, "/v3/dir/download/shared", v3DirDownloadRequest{
		UUID: uuid,
	})
}

func (c *Client) PostV3DirDownloadLinked(ctx context.Context, uuid, linkUUID, linkHasPassword, linkSalt string) (*V3DirDownloadResponse, error) {
	panic("todo")
}
