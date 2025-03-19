package client

import "context"

type V3UserInfoResponse struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	IsPremium   int    `json:"isPremium"`
	MaxStorage  int    `json:"maxStorage"`
	UsedStorage int    `json:"storageUsed"`
	AvatarURL   string `json:"avatarURL"`
	BaseFolder  string `json:"baseFolderUUID"`
}

func (c *Client) GetV3UserInfo(ctx context.Context) (*V3UserInfoResponse, error) {
	var res V3UserInfoResponse
	_, err := c.RequestData(ctx, "GET", GatewayURL("/v3/user/info"), nil, &res)
	return &res, err
}
