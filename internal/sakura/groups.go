package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) CreateGroup(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/groups", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Group](resp)
}

func (c *httpClient) ListGroups(ctx context.Context) ([]Group, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/groups", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Group](resp)
}

func (c *httpClient) DeleteGroup(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/groups/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) SetRouteAuthorization(ctx context.Context, serviceID, routeID string, auth RouteAuthorization) error {
	path := "/services/" + serviceID + "/routes/" + routeID + "/authorization"
	resp, err := c.doRequest(ctx, http.MethodPut, path, auth)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) GetRouteAuthorization(ctx context.Context, serviceID, routeID string) (*RouteAuthorization, error) {
	path := "/services/" + serviceID + "/routes/" + routeID + "/authorization"
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[RouteAuthorization](resp)
}
