package client

import (
	"context"
	"fmt"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type V3DirLinkAddRequest struct {
	UUID       string                 `json:"uuid"`
	ParentUUID string                 `json:"parent"`
	LinkUUID   string                 `json:"linkUUID"`
	ItemType   string                 `json:"type"`
	Metadata   crypto.EncryptedString `json:"metadata"`
	LinkKey    crypto.EncryptedString `json:"key"`
	Expiration string                 `json:"expiration"`
}

func (c *Client) PostV3DirLinkAdd(ctx context.Context, request V3DirLinkAddRequest) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/dir/link/add"), request)
	if err != nil {
		return fmt.Errorf("PostV3DirLinkAdd: %w", err)
	}
	return err
}
