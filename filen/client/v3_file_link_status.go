package client

import "context"

type V3FileLinkStatusResponse struct {
	LinkUUID       string `json:"uuid"`
	Enabled        bool   `json:"enabled"`
	Expiration     int    `json:"expiration"`
	ExpirationText string `json:"expirationText"`
	DownloadBtn    int    `json:"downloadBtn"`
	Password       string `json:"password"`
}

type v3FileLinkStatusRequest struct {
	UUID string `json:"uuid"`
}

func (c *Client) V3FileLinkStatus(ctx context.Context, uuid string) (*V3FileLinkStatusResponse, error) {
	var response V3FileLinkStatusResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/file/link/status"), &v3FileLinkStatusRequest{
		UUID: uuid,
	}, &response)
	if err != nil {
		return nil, err
	}
	return &response, nil
}
