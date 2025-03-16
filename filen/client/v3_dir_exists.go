package client

import "context"

type V3DirExistsResponse struct {
	Exists bool   `json:"exists"`
	UUID   string `json:"uuid"`
}

type v3DirExistsRequest struct {
	NameHashed string `json:"nameHashed"`
	ParentUUID string `json:"parent"`
}

func (c *Client) PostV3DirExists(ctx context.Context, nameHashed string, parentUUID string) (*V3DirExistsResponse, error) {
	var resp V3DirExistsResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/dir/exists"), v3DirExistsRequest{
		NameHashed: nameHashed,
		ParentUUID: parentUUID,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
