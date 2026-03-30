package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) CreateService(ctx context.Context, req CreateServiceRequest) (*Service, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/services", req)
	if err != nil {
		return nil, err
	}
	result, err := decodeResponse[serviceResponse](resp)
	if err != nil {
		return nil, err
	}
	return &result.Service, nil
}

func (c *httpClient) GetService(ctx context.Context, id string) (*Service, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/services/"+id, nil)
	if err != nil {
		return nil, err
	}
	result, err := decodeResponse[serviceResponse](resp)
	if err != nil {
		return nil, err
	}
	return &result.Service, nil
}

func (c *httpClient) ListServices(ctx context.Context) ([]Service, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/services", nil)
	if err != nil {
		return nil, err
	}
	result, err := decodeResponse[serviceListResponse](resp)
	if err != nil {
		return nil, err
	}
	return result.Services, nil
}

func (c *httpClient) UpdateService(ctx context.Context, id string, req UpdateServiceRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/services/"+id, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) DeleteService(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/services/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
