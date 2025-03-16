package client

import "context"

type V3UserPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

type v3UserPublicKeyRequest struct {
	Email string `json:"email"`
}

func (c *Client) PostV3UserPublicKey(ctx context.Context, email string) (*V3UserPublicKeyResponse, error) {
	var resp V3UserPublicKeyResponse
	_, err := c.RequestData(ctx, "POST", GatewayURL("/v3/user/publicKey"), v3UserPublicKeyRequest{
		Email: email,
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
