package client

import (
	"context"
	"fmt"
)

type v3DirSharedRequest struct {
	UUID string `json:"uuid"`
}

type V3DirSharedUser struct {
	Email     string `json:"email"`
	PublicKey string `json:"publicKey"`
}

type V3DirSharedResponse struct {
	Shared bool              `json:"shared"`
	Users  []V3DirSharedUser `json:"users"`
}

func (c *Client) PostV3DirShared(ctx context.Context, dirUUID string) (*V3DirSharedResponse, error) {
	var res V3DirSharedResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/shared"), v3DirSharedRequest{
		UUID: dirUUID,
	}, &res)
	if err != nil {
		return nil, fmt.Errorf("PostV3DirShared: %w", err)
	}
	return &res, nil
}
