package client

import (
	"context"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
)

type v3DirLinkedRequest struct {
	UUID string `json:"uuid"`
}

type V3DirSharedLink struct {
	UUID string                 `json:"linkUUID"`
	Key  crypto.EncryptedString `json:"linkKey"`
}

type V3DirLinkedResponse struct {
	Linked bool              `json:"link"`
	Links  []V3DirSharedLink `json:"links"`
}

func (c *Client) PostV3DirLinked(ctx context.Context, dirUUID string) (*V3DirLinkedResponse, error) {
	var res V3DirLinkedResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/linked"), v3DirLinkedRequest{
		UUID: dirUUID,
	}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
