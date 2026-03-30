package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) CreateCertificate(ctx context.Context, req CreateCertificateRequest) (*Certificate, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/certificates", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Certificate](resp)
}

func (c *httpClient) ListCertificates(ctx context.Context) ([]Certificate, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/certificates", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Certificate](resp)
}

func (c *httpClient) UpdateCertificate(ctx context.Context, id string, req UpdateCertificateRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/certificates/"+id, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) DeleteCertificate(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/certificates/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
