package client

import "context"

type v3ItemSharedRequest struct {
	UUID string `json:"uuid"`
}

type V3ItemSharedUser struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	PublicKey string `json:"publicKey"`
}

type V3ItemSharedResponse struct {
	Shared bool               `json:"sharing"`
	Users  []V3ItemSharedUser `json:"users"`
}

func (c *Client) PostV3ItemShared(ctx context.Context, uuid string) (*V3ItemSharedResponse, error) {
	var res V3ItemSharedResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/item/shared"), v3ItemSharedRequest{
		UUID: uuid,
	}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
