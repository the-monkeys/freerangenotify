package database

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"go.uber.org/zap"
)

// ElasticsearchClient wraps the Elasticsearch client with additional functionality
type ElasticsearchClient struct {
	Client *elasticsearch.Client
	Config *config.DatabaseConfig
	Logger *zap.Logger
}

// NewElasticsearchClient creates a new Elasticsearch client with configuration
func NewElasticsearchClient(cfg *config.Config, logger *zap.Logger) (*ElasticsearchClient, error) {
	// Elasticsearch client configuration
	esConfig := elasticsearch.Config{
		Addresses: cfg.Database.URLs,
		Username:  cfg.Database.Username,
		Password:  cfg.Database.Password,
	}

	// Create client
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	esClient := &ElasticsearchClient{
		Client: client,
		Config: &cfg.Database,
		Logger: logger,
	}

	// Test connection
	if err := esClient.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping Elasticsearch: %w", err)
	}

	logger.Info("Elasticsearch client connected successfully",
		zap.Strings("addresses", cfg.Database.URLs))

	return esClient, nil
}

// GetClient returns the underlying Elasticsearch client
func (es *ElasticsearchClient) GetClient() *elasticsearch.Client {
	return es.Client
}

// Ping checks if Elasticsearch is reachable
func (es *ElasticsearchClient) Ping(ctx context.Context) error {
	req := esapi.PingRequest{}
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ping failed with status: %s", res.Status())
	}

	return nil
}

// Health checks the cluster health
func (es *ElasticsearchClient) Health(ctx context.Context) (*ClusterHealth, error) {
	req := esapi.ClusterHealthRequest{
		WaitForStatus: "yellow",
		Timeout:       time.Duration(es.Config.Timeout) * time.Second,
	}

	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, fmt.Errorf("health request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("health check failed with status: %s", res.Status())
	}

	// Parse health response (simplified)
	health := &ClusterHealth{
		Status:    "green", // This should be parsed from actual response
		NodeCount: 1,
		DataNodes: 1,
	}

	return health, nil
}

// GetIndexName returns the formatted index name with prefix
func (es *ElasticsearchClient) GetIndexName(indexType string) string {
	return fmt.Sprintf("%s_%s", es.Config.IndexPrefix, indexType)
}

// Close closes the client connection
func (es *ElasticsearchClient) Close() error {
	// Elasticsearch client doesn't need explicit closing in v8
	es.Logger.Info("Elasticsearch client connection closed")
	return nil
}

// ClusterHealth represents the cluster health status
type ClusterHealth struct {
	Status    string `json:"status"`
	NodeCount int    `json:"number_of_nodes"`
	DataNodes int    `json:"number_of_data_nodes"`
}

// ConnectionPool manages multiple Elasticsearch connections
type ConnectionPool struct {
	clients []*ElasticsearchClient
	config  *config.DatabaseConfig
	logger  *zap.Logger
	current int
}

// NewConnectionPool creates a new connection pool
// TODO: Implement proper connection pooling
/*
func NewConnectionPool(cfg *config.DatabaseConfig, logger *zap.Logger, poolSize int) (*ConnectionPool, error) {
	// Implementation temporarily disabled
	return nil, fmt.Errorf("connection pooling not yet implemented")
}
*/

// GetClient returns a client from the pool (round-robin)
func (cp *ConnectionPool) GetClient() *ElasticsearchClient {
	client := cp.clients[cp.current]
	cp.current = (cp.current + 1) % len(cp.clients)
	return client
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() error {
	for i, client := range cp.clients {
		if err := client.Close(); err != nil {
			cp.logger.Error("Failed to close connection",
				zap.Int("connection_id", i), zap.Error(err))
		}
	}
	return nil
}
