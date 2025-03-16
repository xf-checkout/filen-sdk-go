package client

import "context"

type v3DirMoveRequest struct {
	UUID          string `json:"uuid"`
	NewParentUUID string `json:"to"`
}

func (c *Client) PostV3DirMove(ctx context.Context, uuid string, newParentUUID string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/dir/move"), v3DirMoveRequest{
		UUID:          uuid,
		NewParentUUID: newParentUUID,
	})
	return err
}
