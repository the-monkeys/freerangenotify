# Topics and subscriptions

Topics group recipients inside one app/environment. The deployment must enable topics. Use an application API key, not a dashboard JWT. Create recipients first using [Users](/docs/users).

Creation requires **`key`**. Save the returned `data.id` as `TOPIC_ID`. The send/broadcast selector is named `topic_key`; it is a different field in a different DTO.

## Create topic

Create a topic and save `data.id` as `TOPIC_ID`.

<!-- api-example:create-topic:request -->
```json
{
  "name": "Product updates",
  "key": "product-updates"
}
```

<!-- api-example:create-topic:curl -->
```bash
curl -sS -X POST "$BASE_URL/topics" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "name": "Product updates",
  "key": "product-updates"
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:create-topic:response -->
```json
{
  "success": true,
  "data": {
    "id": "${TOPIC_ID}",
    "key": "product-updates"
  }
}
```

## Subscribe topic

Subscribe one or more users with an array named `user_ids`.

<!-- api-example:subscribe-topic:request -->
```json
{
  "user_ids": [
    "${USER_ID}"
  ]
}
```

<!-- api-example:subscribe-topic:curl -->
```bash
curl -sS -X POST "$BASE_URL/topics/${TOPIC_ID}/subscribers" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_ids": [
    "${USER_ID}"
  ]
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:subscribe-topic:response -->
```json
{
  "success": true,
  "added": 1
}
```

Remove subscribers with `DELETE /topics/TOPIC_ID/subscribers` and the same `user_ids` body. List with `GET /topics/TOPIC_ID/subscribers`. Read by key with `GET /topics/key/product-updates`. For broadcasts use `topic_key` alongside a valid template or direct content; normal preferences and billing still apply.
