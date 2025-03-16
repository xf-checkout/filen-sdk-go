package client

import "context"

type V3DirLinkStatusResponse struct {
	Exists bool `json:"exists"`

	// the below are only available if exists is true
	// we dream of sum types
	UUID           string `json:"uuid"`
	Key            string `json:"key"`
	Expiration     int    `json:"expiration"`
	ExpirationText string `json:"expirationText"`
	DownloadBtn    int    `json:"downloadBtn"`
	Password       string `json:"password"`
}

type v3DirLinkStatusRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) PostV3DirLinkStatus(ctx context.Context, uuid string) (*V3DirLinkStatusResponse, error) {
	var res V3DirLinkStatusResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/link/status"), v3DirLinkStatusRequest{
		UUID: uuid,
	}, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
