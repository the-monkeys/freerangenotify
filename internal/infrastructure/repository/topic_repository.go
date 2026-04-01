package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"go.uber.org/zap"
)

// TopicRepository implements topic.Repository using Elasticsearch.
type TopicRepository struct {
	topics        *BaseRepository
	subscriptions *BaseRepository
}

// NewTopicRepository creates a new topic repository.
func NewTopicRepository(client *elasticsearch.Client, logger *zap.Logger) topic.Repository {
	return &TopicRepository{
		topics:        NewBaseRepository(client, "topics", logger, RefreshWaitFor),
		subscriptions: NewBaseRepository(client, "topic_subscriptions", logger, RefreshWaitFor),
	}
}

// --- Topic CRUD ---

func (r *TopicRepository) Create(ctx context.Context, t *topic.Topic) error {
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = time.Now().UTC()
	return r.topics.Create(ctx, t.ID, t)
}

func (r *TopicRepository) GetByID(ctx context.Context, id string) (*topic.Topic, error) {
	doc, err := r.topics.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var t topic.Topic
	if err := mapToStruct(doc, &t); err != nil {
		return nil, fmt.Errorf("failed to map document to topic: %w", err)
	}
	return &t, nil
}

func (r *TopicRepository) GetByKey(ctx context.Context, appID, key string) (*topic.Topic, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"key": key}},
				},
			},
		},
		"size": 1,
	}

	result, err := r.topics.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, fmt.Errorf("topic not found with key %s in app %s", key, appID)
	}

	var t topic.Topic
	if err := mapToStruct(result.Hits[0], &t); err != nil {
		return nil, fmt.Errorf("failed to map document to topic: %w", err)
	}
	return &t, nil
}

func (r *TopicRepository) List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*topic.Topic, int64, error) {
	if limit <= 0 {
		limit = 50
	}

	var filters []map[string]interface{}
	if len(linkedIDs) > 0 {
		filters = append(filters, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"terms": map[string]interface{}{"topic_id": linkedIDs}},
				},
				"minimum_should_match": 1,
			},
		})
	} else {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"app_id": appID},
		})
	}
	if environmentID != "" && environmentID != "default" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"environment_id": environmentID},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
		"from": offset,
		"size": limit,
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
	}

	result, err := r.topics.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	topics := make([]*topic.Topic, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var t topic.Topic
		if err := mapToStruct(hit, &t); err != nil {
			continue
		}
		topics = append(topics, &t)
	}
	return topics, result.Total, nil
}

func (r *TopicRepository) Update(ctx context.Context, t *topic.Topic) error {
	t.UpdatedAt = time.Now().UTC()
	return r.topics.Update(ctx, t.ID, t)
}

func (r *TopicRepository) Delete(ctx context.Context, id string) error {
	// Delete all subscriptions for this topic first
	deleteQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"topic_id": id},
		},
	}
	_ = r.subscriptions.DeleteByQuery(ctx, deleteQuery)

	// Delete the topic itself
	return r.topics.Delete(ctx, id)
}

// --- Subscription management ---

func (r *TopicRepository) AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	docs := make(map[string]interface{}, len(userIDs))
	for _, userID := range userIDs {
		subID := fmt.Sprintf("%s_%s", topicID, userID)
		docs[subID] = &topic.TopicSubscription{
			ID:        subID,
			TopicID:   topicID,
			AppID:     appID,
			UserID:    userID,
			CreatedAt: time.Now().UTC(),
		}
	}
	return r.subscriptions.BulkCreate(ctx, docs)
}

func (r *TopicRepository) RemoveSubscribers(ctx context.Context, topicID string, userIDs []string) error {
	for _, userID := range userIDs {
		subID := fmt.Sprintf("%s_%s", topicID, userID)
		if err := r.subscriptions.Delete(ctx, subID); err != nil {
			// Ignore not-found errors during removal
			continue
		}
	}
	return nil
}

func (r *TopicRepository) GetSubscribers(ctx context.Context, topicID string, limit, offset int) ([]topic.TopicSubscription, int64, error) {
	if limit <= 0 {
		limit = 50
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"topic_id": topicID},
		},
		"from": offset,
		"size": limit,
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
	}

	result, err := r.subscriptions.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	subs := make([]topic.TopicSubscription, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var sub topic.TopicSubscription
		if err := mapToStruct(hit, &sub); err != nil {
			continue
		}
		subs = append(subs, sub)
	}
	return subs, result.Total, nil
}

func (r *TopicRepository) GetSubscriberCount(ctx context.Context, topicID string) (int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"topic_id": topicID},
		},
	}
	return r.subscriptions.Count(ctx, query)
}

func (r *TopicRepository) GetUserTopics(ctx context.Context, appID, userID string) ([]*topic.Topic, error) {
	// First get all subscriptions for this user in this app
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"size": 1000,
	}

	result, err := r.subscriptions.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	// Collect topic IDs
	topicIDs := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var sub topic.TopicSubscription
		if err := mapToStruct(hit, &sub); err != nil {
			continue
		}
		topicIDs = append(topicIDs, sub.TopicID)
	}

	if len(topicIDs) == 0 {
		return []*topic.Topic{}, nil
	}

	// Batch fetch all topics in a single query
	topicQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{
				"_id": topicIDs,
			},
		},
		"size": len(topicIDs),
	}
	topicResult, err := r.topics.Search(ctx, topicQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch topics: %w", err)
	}
	topics := make([]*topic.Topic, 0, len(topicResult.Hits))
	for _, hit := range topicResult.Hits {
		var t topic.Topic
		if err := mapToStruct(hit, &t); err != nil {
			continue
		}
		topics = append(topics, &t)
	}
	return topics, nil
}

func (r *TopicRepository) IsSubscribed(ctx context.Context, topicID, userID string) (bool, error) {
	subID := fmt.Sprintf("%s_%s", topicID, userID)
	_, err := r.subscriptions.GetByID(ctx, subID)
	if err != nil {
		return false, nil // Not found = not subscribed
	}
	return true, nil
}
