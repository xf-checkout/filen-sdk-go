package client

import "context"

type v3SearchAddRequest struct {
	Items []V3SearchAddItem `json:"items"`
}

type V3SearchAddItem struct {
	UUID string `json:"uuid"`
	Hash string `json:"hash"`
	Type string `json:"type"`
}

func (c *Client) PostV3SearchAdd(ctx context.Context, items []V3SearchAddItem) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/search/add"), v3SearchAddRequest{
		Items: items,
	})
	return err
}
