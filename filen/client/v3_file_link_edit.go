package client

import (
	"context"
	"encoding/hex"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/crypto"
	"github.com/FilenCloudDienste/filen-sdk-go/filen/types"
	"github.com/google/uuid"
)

type v3FileLinkEditRequest struct {
	LinkUUID       string `json:"uuid"`
	FileUUID       string `json:"fileUUID"`
	Expiration     string `json:"expiration"`
	Password       string `json:"password"`
	PasswordHashed string `json:"passwordHashed"`
	DownloadBtn    bool   `json:"downloadBtn"`
	Type           string `json:"type"`
	Salt           string `json:"salt"`
}

func (c *Client) postV3FileLinkEdit(ctx context.Context, request v3FileLinkEditRequest) error {
	_, err := c.Request(ctx, "POST", GatewayURL("/v3/file/link/edit"), request)
	return err
}

//func (c *Client) PostV3FileLinkDelete(ctx context.Context, uuid string) error {
//
//}

func (c *Client) PostV3FileLinkEditEnable(ctx context.Context, file types.File) (string, error) {
	linkUUID := uuid.NewString()
	err := c.postV3FileLinkEdit(ctx, v3FileLinkEditRequest{
		LinkUUID:       linkUUID,
		FileUUID:       file.UUID,
		Expiration:     "never",
		Password:       "empty",
		PasswordHashed: crypto.V2Hash([]byte("empty")),
		DownloadBtn:    false,
		Type:           "enable",
		Salt:           hex.EncodeToString(crypto.GenerateRandomBytes(16)),
	})
	if err != nil {
		return "", err
	}
	return linkUUID, nil
}
