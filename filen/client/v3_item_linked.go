package client

import "context"

type v3ItemLinkedRequest struct {
	UUID string `json:"uuid"`
}

type V3ItemLinkedLink struct {
	LinkUUID string `json:"linkUUID"`
	Key      string `json:"key"`
}

type V3ItemLinkedResponse struct {
	Linked bool               `json:"link"`
	Links  []V3ItemLinkedLink `json:"links"`
}

func (c *Client) PostV3ItemLinked(ctx context.Context, uuid string) (*V3ItemLinkedResponse, error) {
	var res V3ItemLinkedResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/item/linked"), v3ItemLinkedRequest{
		UUID: uuid,
	}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
