package client

import "context"

type V3UserLockRequest struct {
	LockUUID string `json:"uuid"`
	Type     string `json:"type"`
	Resource string `json:"resource"`
}

type V3UserLockResponse struct {
	Acquired  bool   `json:"acquired"`
	Released  bool   `json:"released"`
	Refreshed bool   `json:"refreshed"`
	Resource  string `json:"resource"`
	Status    string `json:"status"`
}

func (c *Client) PostV3UserLock(ctx context.Context, req V3UserLockRequest) (*V3UserLockResponse, error) {
	var res V3UserLockResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/user/lock"), req, &res)
	return &res, err
}
