#!/bin/bash
# Verify scheduled notifications and workflow schedules from Elasticsearch
# Usage: ./scripts/verify-schedules.sh [ES_URL]
# Default ES: http://localhost:9200

ES_URL="${1:-http://localhost:9200}"

echo "=== Elasticsearch: $ES_URL ==="
echo ""

# 1. Notifications with scheduled_at (from notification tab broadcast)
echo "--- Notifications with scheduled_at (notification tab broadcasts) ---"
curl -s -X POST "$ES_URL/notifications/_search?pretty" -H "Content-Type: application/json" -d '{
  "query": {
    "bool": {
      "must": [
        {"exists": {"field": "scheduled_at"}}
      ]
    }
  },
  "sort": [{"scheduled_at": "desc"}],
  "size": 50,
  "_source": ["notification_id", "user_id", "channel", "status", "scheduled_at", "sent_at", "created_at", "title", "app_id"]
}' | head -120

echo ""
echo "--- Count: notifications with scheduled_at ---"
curl -s -X POST "$ES_URL/notifications/_count?pretty" -H "Content-Type: application/json" -d '{
  "query": {"exists": {"field": "scheduled_at"}}
}'

echo ""
echo ""

# 2. Workflow schedules (cron - 10 PM daily = 0 22 * * *)
echo "--- Workflow Schedules (cron jobs) ---"
curl -s -X POST "$ES_URL/workflow_schedules/_search?pretty" -H "Content-Type: application/json" -d '{
  "query": {"match_all": {}},
  "sort": [{"created_at": "desc"}],
  "size": 20,
  "_source": ["id", "name", "app_id", "workflow_trigger_id", "cron", "target_type", "status", "last_run_at", "created_at"]
}' | head -80

echo ""
echo "--- Count: workflow schedules ---"
curl -s -X GET "$ES_URL/workflow_schedules/_count?pretty"

echo ""
echo ""

# 3. Recent notifications (all) - to see 9:40/9:41 activity
echo "--- Recent notifications (last 24h by created_at) ---"
FROM_DATE=$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%S 2>/dev/null || date -u -v-24H +%Y-%m-%dT%H:%M:%S 2>/dev/null)
curl -s -X POST "$ES_URL/notifications/_search?pretty" -H "Content-Type: application/json" -d "{
  \"query\": {
    \"range\": {
      \"created_at\": {\"gte\": \"$FROM_DATE\"}
    }
  },
  \"sort\": [{\"created_at\": \"desc\"}],
  \"size\": 30,
  \"_source\": [\"notification_id\", \"user_id\", \"channel\", \"status\", \"scheduled_at\", \"sent_at\", \"created_at\", \"title\"]
}" | head -100
