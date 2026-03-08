# Templates

Templates define the content of your notifications. They support dynamic variables, versioning, and multi-channel formatting.

## Creating Templates

Templates belong to an Application and target a specific channel. Required fields:

| Field | Type | Description |
|-------|------|-------------|
| `app_id` | string | The application this template belongs to |
| `name` | string | Unique identifier (e.g., `order_confirmation`) |
| `channel` | string | Target channel: `email`, `webhook`, `push`, `sms`, `sse` |
| `body` | string | Template content with variable placeholders |
| `subject` | string | Email subject line (email channel only) |
| `variables` | string[] | List of expected variable names |
| `locale` | string | Language code (default: `en`) |

### Variable Syntax

Use Go template syntax for dynamic content:

```
Hello {{.user_name}}, your order #{{.order_id}} has shipped!
```

Variables are passed in the `data` field when sending a notification:

```json
{
  "data": {
    "user_name": "Alice",
    "order_id": "ORD-12345"
  }
}
```

## Versioning

Every template edit creates a new **version**. This gives you a full audit trail and the ability to compare and rollback.

### Viewing Version History

Open any template and click **Version History** to see all previous versions with timestamps and authors.

### Comparing Versions (Diff)

Click **Compare** to open the diff viewer. It shows a side-by-side comparison of the body, subject, and variables between any two versions.

### Rolling Back

Select a previous version and click **Rollback** to restore it as the current version. This creates a new version (so you never lose history).

## Template Library

FreeRangeNotify includes pre-built templates you can clone and customize:

- Welcome email
- Password reset
- Order confirmation
- Account verification
- Alert notification

Clone from the **Template Library** tab in your application.

## Testing Templates

### Render Preview

Test your template with sample data before sending:

```bash
curl -X POST http://localhost:8080/v1/templates/TEMPLATE_ID/render \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"data": {"user_name": "Test User", "order_id": "ORD-00001"}}'
```

### Test Send

Use the **Test Panel** in the template detail view to render and send a test notification with sample data.

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/templates/` | Create a new template |
| GET | `/v1/templates/` | List all templates for the app |
| GET | `/v1/templates/:id` | Get template details |
| PUT | `/v1/templates/:id` | Update template (creates new version) |
| DELETE | `/v1/templates/:id` | Delete a template |
| POST | `/v1/templates/:id/render` | Render template with sample data |
| GET | `/v1/templates/:id/versions` | List version history |
| POST | `/v1/templates/:id/rollback` | Rollback to a previous version |
