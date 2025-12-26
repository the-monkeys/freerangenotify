# Guide: Testing Smart Notification Delivery End-to-End

This guide provides step-by-step instructions to verify the "Smart Notification Delivery" system, including user presence, instant delivery (flush), and dynamic routing.

## 1. Prerequisites & Environment Setup

Ensure your environment is clean and the core services are running.

### Start the Hub (Docker Compose)
Run this in your main project terminal:
```powershell
# Rebuild and start core services
docker-compose down
docker-compose build
docker-compose up -d

# Verify all services are healthy
docker ps --format "{{.Names}}: {{.Status}}"
```

### Obtain Your Credentials
You will need an **API Key** and your **User IDs**.
- **API Key**: Found in `api_key.txt` (Example: `frn_1766408682`)
- **App ID**: `8b6b999a-ee3f-47b1-9f2a-7c816b9bf30f`

| User | ID | Configured Port |
| :--- | :--- | :--- |
| Alice | `2d036df1-66a5-4e53-bf8f-95ac34f38bfa` | 8091 |
| Bob | `e6eab7c1-1d51-4bc6-a406-ebd91c816e4e` | 8092 |
| Charlie | `8da4b87d-e267-4eac-9c9d-8295a7d146c2` | 8093 |

---

## 2. Testing "Instant Flush" (User Offline -> Online)

This test proves that notifications sent while a user is offline are delivered **immediately** once they log in.

### Step A: Send Notification While Offline
Ensure Alice's receiver is NOT running. Send a notification:
```powershell
# Send via inline PowerShell
$body = @{user_id='2d036df1-66a5-4e53-bf8f-95ac34f38bfa'; channel='webhook'; priority='high'; title='Fresh Alice'; body='Hi Alice, smart delivery test!'; template_id='e3a0ea73-d4d0-40db-8c34-d179e4c0de43'; data=@{name='Alice'}} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/v1/notifications" -Method Post -Headers @{Authorization="Bearer frn_1766408682"} -Body $body -ContentType "application/json"
```
Verify status is `queued` and check logs for "connection refused":
```powershell
# Replace with the ID from the output above
curl.exe -s -H "Authorization: Bearer frn_1766408682" http://localhost:8080/v1/notifications/<NOTIF_ID>
```

### Step B: Start Receiver (Check-in)
Open a **NEW terminal** and start Alice's receiver:
```powershell
go run ./cmd/receiver/main.go --port 8091 --userid 2d036df1-66a5-4e53-bf8f-95ac34f38bfa --apikey frn_1766408682
```
**Observation**: Within 1 second of seeing "Check-in SUCCESSFUL", the terminal should print "SUCCESS: Received notification".

---

## 3. Testing "Dynamic Routing" (Port Override)

This test proves the Hub can deliver to a receiver running on a **temporary/dynamic port**, ignoring the static port in the user's profile.

### Step A: Start Receiver on a Dynamic Port
Alice's profile is set to port `8091`. Start her receiver on port **8095** instead.
```powershell
# In a new terminal
go run ./cmd/receiver/main.go --port 8095 --userid 2d036df1-66a5-4e53-bf8f-95ac34f38bfa --apikey frn_1766408682
```

### Step B: Send Notification
Send another notification to Alice:
```powershell
# Send via inline PowerShell
$body = @{user_id='2d036df1-66a5-4e53-bf8f-95ac34f38bfa'; channel='webhook'; priority='high'; title='Fresh Alice'; body='Hi Alice, smart delivery test!'; template_id='e3a0ea73-d4d0-40db-8c34-d179e4c0de43'; data=@{name='Alice'}} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/v1/notifications" -Method Post -Headers @{Authorization="Bearer frn_1766408682"} -Body $body -ContentType "application/json"
```

**Observation**: The notification is delivered to the terminal running on port **8095**. The Worker logs will show: `Smart Delivery: Overriding static webhook with dynamic URL`.

---

## 4. Multi-User Simultaneous Test

Testing multiple users in parallel terminals to ensure isolation.

### Terminal 1 (Alice)
```powershell
go run ./cmd/receiver/main.go --port 8091 --userid 2d036df1-66a5-4e53-bf8f-95ac34f38bfa --apikey frn_1766408682
```

### Terminal 2 (Bob)
```powershell
go run ./cmd/receiver/main.go --port 8092 --userid e6eab7c1-1d51-4bc6-a406-ebd91c816e4e --apikey frn_1766408682
```

### Terminal 3 (Charlie)
```powershell
go run ./cmd/receiver/main.go --port 8093 --userid 8da4b87d-e267-4eac-9c9d-8295a7d146c2 --apikey frn_1766408682
```

### Step D: Send Broadcast
You can now send notifications to any of these users and watch their respective terminals light up instantly.

---

## 5. Debugging & Logs

If notifications are not arriving:
1. **Check Presence**: Ensure the user is registered in Redis.
2. **Worker Logs**: `docker-compose logs -f notification-worker`
3. **Queue Status**: Check the Admin API `GET /v1/admin/queues` to see if items are stuck.
