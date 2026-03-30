package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) CreateOIDC(ctx context.Context, req CreateOIDCRequest) (*OIDCConfig, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/oidc", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[OIDCConfig](resp)
}

func (c *httpClient) GetOIDC(ctx context.Context, id string) (*OIDCConfig, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/oidc/"+id, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[OIDCConfig](resp)
}

func (c *httpClient) ListOIDC(ctx context.Context) ([]OIDCConfig, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/oidc", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[OIDCConfig](resp)
}

func (c *httpClient) DeleteOIDC(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/oidc/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
