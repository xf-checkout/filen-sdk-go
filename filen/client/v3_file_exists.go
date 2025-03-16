package client

import "context"

type V3FileExistsResponse struct {
	Exists bool   `json:"exists"`
	UUID   string `json:"uuid"`
}

type v3FileExistsRequest struct {
	NameHashed string `json:"nameHashed"`
	ParentUUID string `json:"parent"`
}

func (c *Client) PostV3FileExists(ctx context.Context, nameHashed string, parentUUID string) (*V3FileExistsResponse, error) {
	var resp V3FileExistsResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/file/exists"), v3FileExistsRequest{
		NameHashed: nameHashed,
		ParentUUID: parentUUID,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
