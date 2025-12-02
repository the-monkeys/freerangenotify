# Quick API Testing Commands

## Setup Complete! ✅
- ✅ Docker services running (Elasticsearch, Redis)
- ✅ Server running on http://localhost:8080
- ✅ All indices created

---

## Option 1: Automated Testing Script
Run the full test suite:
```powershell
.\test_apis.ps1
```

## Option 2: Manual Testing (Step by Step)
Follow the guide in `API_TESTING_TRACKER.md` and use these curl commands:

### Test 1: Create Application
```powershell
curl.exe -X POST http://localhost:8080/v1/apps `
  -H "Content-Type: application/json" `
  -d '{\"app_name\":\"Test App 1\",\"webhook_url\":\"https://example.com/webhook\",\"enable_webhooks\":true,\"enable_analytics\":true}'
```

### Test 2: Get Application (replace {app_id})
```powershell
curl.exe -X GET http://localhost:8080/v1/apps/{app_id}
```

### Test 3: Create User (replace {api_key})
```powershell
curl.exe -X POST http://localhost:8080/v1/users `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer {api_key}" `
  -d '{\"external_user_id\":\"user001\",\"email\":\"user001@example.com\",\"phone\":\"+1234567890\",\"timezone\":\"America/New_York\",\"language\":\"en\"}'
```

---

## Testing Workflow

1. **Run Test** → Execute curl command or use Postman
2. **Check Response** → Verify status code and response body
3. **Record Result** → Update `API_TESTING_TRACKER.md` with:
   - ✅ or ❌ in the header
   - Status code
   - Response body
   - Any notes
4. **If Failed** → Debug and fix code
5. **Update Documentation** → Mark as documented when API works correctly
6. **Save Resource IDs** → Record all created IDs in the tracker tables

---

## Resource ID Tracking

After each create operation, save these IDs:

- **App ID** → From Create Application response
- **API Key** → From Create Application response (needed for all other APIs)
- **User ID** → From Create User response
- **Device ID** → From Add Device response
- **Notification ID** → From Send Notification response
- **Template ID** → From Create Template response

---

## Next Steps

Choose your testing method:

**A) Automated (Recommended for first run):**
```powershell
.\test_apis.ps1
```
This will test all APIs sequentially with pauses between each test.

**B) Manual (Recommended for documentation):**
1. Open `API_TESTING_TRACKER.md`
2. Test APIs one by one using curl or Postman
3. Record results after each test
4. Update documentation as you go

---

## Stopping the Server

When done testing:
```powershell
# Find server process
Get-Process | Where-Object {$_.ProcessName -eq "server"}

# Stop server
Stop-Process -Name "server" -Force

# Restart Docker container if needed
docker start freerange-notification-service
```

---

## Documentation to Update

After each successful API test, update:
- `API_TESTING_TRACKER.md` - Test results
- `docs/swagger.yaml` or Swagger annotations - API documentation

---

**Ready to start?** Choose option A or B above! 🚀
