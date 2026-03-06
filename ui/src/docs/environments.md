# Multi-Environment Setup

Environments let you isolate notification configurations across development, staging, and production. Each environment gets its own API key and can have independent provider settings.

## Overview

When you create an Application, a **default** environment is automatically created. You can add additional environments to separate testing from production traffic.

| Environment | Use Case |
|-------------|----------|
| `development` | Local testing, debugging |
| `staging` | Pre-production validation |
| `production` | Live user notifications |

## Creating Environments

### Via the Dashboard

Navigate to your application → **Environments** tab → **Create Environment**.

### Via the API

```bash
curl -X POST http://localhost:8080/v1/apps/APP_ID/environments \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "staging"}'
```

Each environment returns its own API key. Use the environment-specific key to scope all API calls.

## API Key Scoping

When you use an environment's API key, all operations are scoped to that environment:

- Templates created with a staging key are only visible in staging
- Notifications sent with a production key hit production providers
- Users can exist across environments

## Promoting Resources

Move templates and configurations from one environment to another:

```bash
curl -X POST http://localhost:8080/v1/apps/APP_ID/environments/promote \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source_env_id": "STAGING_ENV_ID",
    "target_env_id": "PRODUCTION_ENV_ID",
    "resources": ["templates"]
  }'
```

This copies all templates from staging to production without affecting staging's state.

## Best Practices

- **Never share API keys between environments** — each key scopes traffic to its environment
- **Test in staging first** — validate templates, workflows, and provider config before promoting
- **Use promotion for deployments** — avoid manual recreation of templates in production
- **Rotate keys regularly** — regenerate API keys periodically for security

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/apps/:id/environments` | Create an environment |
| GET | `/v1/apps/:id/environments` | List all environments |
| GET | `/v1/apps/:id/environments/:env_id` | Get environment details |
| DELETE | `/v1/apps/:id/environments/:env_id` | Delete an environment |
| POST | `/v1/apps/:id/environments/promote` | Promote resources between environments |
