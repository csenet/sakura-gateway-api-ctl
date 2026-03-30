package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/domains", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Domain](resp)
}

func (c *httpClient) ListDomains(ctx context.Context) ([]Domain, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/domains", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Domain](resp)
}

func (c *httpClient) UpdateDomain(ctx context.Context, id string, req UpdateDomainRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/domains/"+id, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) DeleteDomain(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/domains/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
