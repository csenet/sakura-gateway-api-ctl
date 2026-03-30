package sakura

import (
	"context"
	"fmt"
	"net/http"
)

func (c *httpClient) SetRequestTransform(ctx context.Context, serviceID, routeID string, req RequestTransform) error {
	path := fmt.Sprintf("/services/%s/routes/%s/request", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) GetRequestTransform(ctx context.Context, serviceID, routeID string) (*RequestTransform, error) {
	path := fmt.Sprintf("/services/%s/routes/%s/request", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[RequestTransform](resp)
}

func (c *httpClient) SetResponseTransform(ctx context.Context, serviceID, routeID string, req ResponseTransform) error {
	path := fmt.Sprintf("/services/%s/routes/%s/response", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) GetResponseTransform(ctx context.Context, serviceID, routeID string) (*ResponseTransform, error) {
	path := fmt.Sprintf("/services/%s/routes/%s/response", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[ResponseTransform](resp)
}
