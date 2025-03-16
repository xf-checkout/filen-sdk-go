package client

import "context"

type v3FileMoveRequest struct {
	UUID          string `json:"uuid"`
	NewParentUUID string `json:"to"`
}

func (c *Client) PostV3FileMove(ctx context.Context, uuid string, newParentUUID string) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/file/move"), v3FileMoveRequest{
		UUID:          uuid,
		NewParentUUID: newParentUUID,
	})
	return err
}
