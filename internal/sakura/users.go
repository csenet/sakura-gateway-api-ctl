package sakura

import (
	"context"
	"fmt"
	"net/http"
)

func (c *httpClient) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/users", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[User](resp)
}

func (c *httpClient) GetUser(ctx context.Context, id string) (*User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users/"+id, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[User](resp)
}

func (c *httpClient) ListUsers(ctx context.Context) ([]User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[User](resp)
}

func (c *httpClient) DeleteUser(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/users/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) SetUserAuthentication(ctx context.Context, userID string, auth UserAuthentication) error {
	path := fmt.Sprintf("/users/%s/authentication", userID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, auth)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) SetUserGroups(ctx context.Context, userID string, groups UserGroups) error {
	path := fmt.Sprintf("/users/%s/groups", userID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, groups)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
