package client

import (
	"context"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3DirMetadataRequest struct {
	UUID       string                 `json:"uuid"`
	NameHashed string                 `json:"nameHashed"`
	Metadata   crypto.EncryptedString `json:"name"`
}

func (c *Client) PostV3DirMetadata(ctx context.Context, uuid string, nameHashed string, metadata crypto.EncryptedString) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/dir/metadata"), v3DirMetadataRequest{
		UUID:       uuid,
		NameHashed: nameHashed,
		Metadata:   metadata,
	})
	if err != nil {
		return fmt.Errorf("post v3 dir metadata: %w", err)
	}
	return nil
}
