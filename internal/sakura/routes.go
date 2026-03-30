package sakura

import (
	"context"
	"fmt"
	"net/http"
)

func (c *httpClient) CreateRoute(ctx context.Context, serviceID string, req CreateRouteRequest) (*Route, error) {
	path := fmt.Sprintf("/services/%s/routes", serviceID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Route](resp)
}

func (c *httpClient) GetRoute(ctx context.Context, serviceID, routeID string) (*Route, error) {
	path := fmt.Sprintf("/services/%s/routes/%s", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Route](resp)
}

func (c *httpClient) ListRoutes(ctx context.Context, serviceID string) ([]Route, error) {
	path := fmt.Sprintf("/services/%s/routes", serviceID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Route](resp)
}

func (c *httpClient) UpdateRoute(ctx context.Context, serviceID, routeID string, req UpdateRouteRequest) error {
	path := fmt.Sprintf("/services/%s/routes/%s", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) DeleteRoute(ctx context.Context, serviceID, routeID string) error {
	path := fmt.Sprintf("/services/%s/routes/%s", serviceID, routeID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
