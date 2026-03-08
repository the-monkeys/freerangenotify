package freerangenotify

import (
	"context"
	"net/url"
	"strconv"
)

// TopicsClient handles topic operations.
type TopicsClient struct {
	client *Client
}

// Create creates a new notification topic.
func (t *TopicsClient) Create(ctx context.Context, params CreateTopicParams) (*Topic, error) {
	var result Topic
	if err := t.client.do(ctx, "POST", "/topics/", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a topic by ID.
func (t *TopicsClient) Get(ctx context.Context, topicID string) (*Topic, error) {
	var result Topic
	if err := t.client.do(ctx, "GET", "/topics/"+topicID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetByKey retrieves a topic by its unique key.
func (t *TopicsClient) GetByKey(ctx context.Context, key string) (*Topic, error) {
	var result Topic
	if err := t.client.do(ctx, "GET", "/topics/key/"+key, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a topic.
func (t *TopicsClient) Delete(ctx context.Context, topicID string) error {
	return t.client.do(ctx, "DELETE", "/topics/"+topicID, nil, nil)
}

// List returns a paginated list of topics.
func (t *TopicsClient) List(ctx context.Context, page, pageSize int) (*TopicListResponse, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result TopicListResponse
	if err := t.client.doWithQuery(ctx, "GET", "/topics/", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AddSubscribers adds users to a topic.
func (t *TopicsClient) AddSubscribers(ctx context.Context, topicID string, userIDs []string) error {
	payload := map[string]interface{}{"user_ids": userIDs}
	return t.client.do(ctx, "POST", "/topics/"+topicID+"/subscribers", payload, nil)
}

// RemoveSubscribers removes users from a topic.
func (t *TopicsClient) RemoveSubscribers(ctx context.Context, topicID string, userIDs []string) error {
	payload := map[string]interface{}{"user_ids": userIDs}
	return t.client.do(ctx, "DELETE", "/topics/"+topicID+"/subscribers", payload, nil)
}

// GetSubscribers returns a paginated list of subscribers for a topic.
func (t *TopicsClient) GetSubscribers(ctx context.Context, topicID string, page, pageSize int) (*SubscriberListResponse, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result SubscriberListResponse
	if err := t.client.doWithQuery(ctx, "GET", "/topics/"+topicID+"/subscribers", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
