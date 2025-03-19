package client

import "context"

type V3DirSizeResponse struct {
	Size    int `json:"size"`
	Folders int `json:"folders"`
	Files   int `json:"files"`
}

type v3dirSizeRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3DirSize(ctx context.Context, uuid string) (*V3DirSizeResponse, error) {
	var res V3DirSizeResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/size"), v3dirSizeRequest{UUID: uuid}, &res)
	return &res, err
}
