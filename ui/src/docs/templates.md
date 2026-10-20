# Templates

Complete [Getting started](/docs/getting-started) first and save `APP_ID` and `TEMPLATE_ID`. Templates use `{{.name}}` variables supplied in notification `data`. Existing bare-variable syntax remains supported. API-key scope determines ownership; use the matching app ID in DTOs and paths.

Create with `POST /templates`, read with `GET /templates/TEMPLATE_ID`, list with `GET /templates`, and delete with `DELETE /templates/TEMPLATE_ID` (204). Deletion does not recall delivered messages. Imported templates follow resource-sharing ownership rules.

## Render template

Render preview checks variables without sending a message.

<!-- api-example:render-template:request -->
```json
{
  "data": {
    "name": "Alice"
  }
}
```

<!-- api-example:render-template:curl -->
```bash
curl -sS -X POST "$BASE_URL/templates/${TEMPLATE_ID}/render" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "data": {
    "name": "Alice"
  }
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:render-template:response -->
```json
{
  "rendered_body": "Hello Alice"
}
```

## Update template

**Update in place.** This leaves version 1 as version 1. It does not automatically preserve an earlier snapshot. Create a snapshot before editing when you need the old content retained.

<!-- api-example:update-template:request -->
```json
{
  "body": "Hi {{.name}}"
}
```

<!-- api-example:update-template:curl -->
```bash
curl -sS -X PUT "$BASE_URL/templates/${TEMPLATE_ID}" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "body": "Hi {{.name}}"
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:update-template:response -->
```json
{
  "id": "${TEMPLATE_ID}",
  "version": 1,
  "body": "Hi {{.name}}"
}
```

## Snapshot template

**Explicit snapshot.** This creates a new version of the current stored content. The legacy DTO requires `body`; send the current body. The endpoint snapshots the stored template rather than applying request content. Save response `id` as `VERSION_ID`.

<!-- api-example:snapshot-template:request -->
```json
{
  "locale": "en",
  "created_by": "backend",
  "body": "Hi {{.name}}"
}
```

<!-- api-example:snapshot-template:curl -->
```bash
curl -sS -X POST "$BASE_URL/templates/${APP_ID}/welcome_email/versions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "locale": "en",
  "created_by": "backend",
  "body": "Hi {{.name}}"
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:snapshot-template:response -->
```json
{
  "id": "${VERSION_ID}",
  "version": 2
}
```

## Template history

History uses the app/name route, with locale filtering. It returns an array.

<!-- api-example:template-history:curl -->
```bash
curl -sS -X GET "$BASE_URL/templates/${APP_ID}/welcome_email/versions?locale=en" \
  -H "Authorization: Bearer $API_KEY"
```

HTTP 200; relevant response fields:

<!-- api-example:template-history:response -->
```json
[
  {
    "version": 1
  },
  {
    "version": 2
  }
]
```

## Rollback template

Rollback creates another version from the selected saved version. It does not delete history. Here version 1 already contains the in-place edit, so the restored body is still `Hi {{.name}}`.

<!-- api-example:rollback-template:request -->
```json
{
  "version": 1,
  "updated_by": "backend"
}
```

<!-- api-example:rollback-template:curl -->
```bash
curl -sS -X POST "$BASE_URL/templates/${VERSION_ID}/rollback" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "version": 1,
  "updated_by": "backend"
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:rollback-template:response -->
```json
{
  "version": 3,
  "body": "Hi {{.name}}"
}
```

