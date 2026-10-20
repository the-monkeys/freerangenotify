# Users and subscribers

Follow [Getting started](/docs/getting-started) to create a user and save `data.user_id` as `USER_ID`. Use `external_id` for your own stable customer ID. Existing internal IDs, email resolution and legacy identifiers continue to work; no production ID migration is required.

## Read

Both API-key header formats work. This example uses `X-API-Key`.

<!-- api-example:get-user:curl -->
```bash
curl -sS -X GET "$BASE_URL/users/${USER_ID}" \
  -H "X-API-Key: $API_KEY"
```

HTTP 200; relevant response fields (other fields omitted):

<!-- api-example:get-user:response -->
```json
{
  "success": true,
  "data": {
    "user_id": "${USER_ID}",
    "external_id": "alice"
  }
}
```

You can also use `GET /users/by-external-id/alice` under the same app key. URL-encode external identifiers.

## Update

Send fields to change. Several empty string fields are ignored by the existing update DTO; don't assume they clear an address.

<!-- api-example:update-user:request -->
```json
{
  "full_name": "Alice Example"
}
```

<!-- api-example:update-user:curl -->
```bash
curl -sS -X PUT "$BASE_URL/users/${USER_ID}" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "full_name": "Alice Example"
}
JSON
```

HTTP 200; relevant response fields (other fields omitted):

<!-- api-example:update-user:response -->
```json
{
  "success": true,
  "data": {
    "user_id": "${USER_ID}",
    "full_name": "Alice Example"
  }
}
```

## Delete

This does not retract provider messages already delivered. Linked resources follow the app's unlinking and ownership-transfer rules.

<!-- api-example:delete-user:curl -->
```bash
curl -sS -X DELETE "$BASE_URL/users/${USER_ID}" \
  -H "Authorization: Bearer $API_KEY"
```

HTTP 200; relevant response fields (other fields omitted):

<!-- api-example:delete-user:response -->
```json
{
  "success": true
}
```

## List, preferences and devices

`GET /users?page=1&page_size=20` returns `data`, `total`, `page` and `page_size`. Filter by email, timezone or language. Use `GET /users/USER_ID/preferences` and `PUT /users/USER_ID/preferences` for channel opt-in, DND and quiet hours.

Use `POST /users/USER_ID/devices` to register a push device, `GET` to list, and `DELETE /users/USER_ID/devices/DEVICE_ID` to remove it. Supply the platform and provider token required by your configuration; see [API reference](/docs/api).

## Bulk creation and CSV import

In **Subscribers → Import CSV**, upload a UTF-8 CSV with a unique `email` header. Optional columns include `full_name`, `phone`, `external_id`, `timezone`, `language` and legacy `user_id`. One file supports **10,000 rows and 10 MiB**. A 6,000-row file does not need manual division. The UI validates rows and sends bounded chunks, reporting created, updated, skipped and failed rows.

`POST /users/bulk` is the backend bulk API. Inspect per-row outcomes; an import is not all-or-nothing. If an acknowledgement is lost, check subscribers before retrying because that chunk may already be written. Import bounds do not imply unlimited HTTP sizes or duplicate-free automatic retries.
