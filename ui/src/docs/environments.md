# Environments

When multi-environment support is enabled, open your app's **Environments** area to manage development, staging and production. App/environment management uses a dashboard JWT and owner authorization. Recipient/template/send APIs use the selected environment's full API key.

Existing app keys keep their default/legacy behavior. Do not assume every existing app has named environments automatically created. Use `GET /apps/APP_ID/environments` to inspect your app.

## Create staging

Create staging and save `data.id` as `STAGING_ENV_ID`. Obtain its full key through the authorized reveal action; ordinary responses mask it.

<!-- api-example:create-staging:request -->
```json
{
  "name": "staging"
}
```

<!-- api-example:create-staging:curl -->
```bash
curl -sS -X POST "$BASE_URL/apps/${APP_ID}/environments" \
  -H "Authorization: Bearer $DASHBOARD_JWT" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "name": "staging"
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:create-staging:response -->
```json
{
  "success": true,
  "data": {
    "id": "${STAGING_ENV_ID}",
    "name": "staging"
  }
}
```

## Create production

Create production only if it does not already exist; save `data.id` as `PRODUCTION_ENV_ID`.

<!-- api-example:create-production:request -->
```json
{
  "name": "production"
}
```

<!-- api-example:create-production:curl -->
```bash
curl -sS -X POST "$BASE_URL/apps/${APP_ID}/environments" \
  -H "Authorization: Bearer $DASHBOARD_JWT" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "name": "production"
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:create-production:response -->
```json
{
  "success": true,
  "data": {
    "id": "${PRODUCTION_ENV_ID}",
    "name": "production"
  }
}
```

## Promote environment

Promotion copies supported resources between environments belonging to the same app. This empty staging example promotes zero templates; inspect the counts. Existing target templates can be updated, so preview your source and target before running this in production.

<!-- api-example:promote-environment:request -->
```json
{
  "source_env_id": "${STAGING_ENV_ID}",
  "target_env_id": "${PRODUCTION_ENV_ID}",
  "resources": [
    "templates"
  ]
}
```

<!-- api-example:promote-environment:curl -->
```bash
curl -sS -X POST "$BASE_URL/apps/${APP_ID}/environments/promote" \
  -H "Authorization: Bearer $DASHBOARD_JWT" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "source_env_id": "${STAGING_ENV_ID}",
  "target_env_id": "${PRODUCTION_ENV_ID}",
  "resources": [
    "templates"
  ]
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:promote-environment:response -->
```json
{
  "success": true,
  "data": {
    "templates_promoted": 0,
    "workflows_promoted": 0
  }
}
```
