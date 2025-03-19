package client

import "context"

func (c *Client) PostV3TrashEmpty(ctx context.Context) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/trash/empty"), nil)
	return err
}
