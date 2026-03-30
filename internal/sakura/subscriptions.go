package sakura

import (
	"context"
	"net/http"
)

func (c *httpClient) ListPlans(ctx context.Context) ([]Plan, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/plans", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Plan](resp)
}

func (c *httpClient) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (*Subscription, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/subscriptions", req)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Subscription](resp)
}

func (c *httpClient) GetSubscription(ctx context.Context, id string) (*Subscription, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/subscriptions/"+id, nil)
	if err != nil {
		return nil, err
	}
	return decodeResponse[Subscription](resp)
}

func (c *httpClient) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	return decodeListResponse[Subscription](resp)
}

func (c *httpClient) UpdateSubscription(ctx context.Context, id string, req UpdateSubscriptionRequest) error {
	resp, err := c.doRequest(ctx, http.MethodPut, "/subscriptions/"+id, req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *httpClient) DeleteSubscription(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/subscriptions/"+id, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
