# FreeRangeNotify API Testing Script
# This script tests all implemented API endpoints

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "FreeRangeNotify API Testing" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

$baseUrl = "http://localhost:8080"

# Helper function to make API calls
function Invoke-APITest {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Endpoint,
        [object]$Body = $null,
        [hashtable]$Headers = @{}
    )
    
    Write-Host "`n--- $Name ---" -ForegroundColor Yellow
    Write-Host "$Method $Endpoint" -ForegroundColor Gray
    
    try {
        $params = @{
            Uri         = "$baseUrl$Endpoint"
            Method      = $Method
            ContentType = "application/json"
            Headers     = $Headers
        }
        
        if ($Body) {
            $params.Body = ($Body | ConvertTo-Json -Depth 10)
            Write-Host "Request Body:" -ForegroundColor Gray
            Write-Host ($Body | ConvertTo-Json -Depth 10) -ForegroundColor DarkGray
        }
        
        $response = Invoke-RestMethod @params
        Write-Host "Response:" -ForegroundColor Green
        Write-Host ($response | ConvertTo-Json -Depth 10) -ForegroundColor White
        return $response
    }
    catch {
        Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails.Message) {
            Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
        return $null
    }
}

# 1. Health Check
Write-Host "`n========== System Endpoints ==========" -ForegroundColor Cyan
Invoke-APITest -Name "Health Check" -Method "GET" -Endpoint "/health"
Invoke-APITest -Name "Version Info" -Method "GET" -Endpoint "/version"
Invoke-APITest -Name "API Status" -Method "GET" -Endpoint "/api/v1/status"
Invoke-APITest -Name "Database Stats" -Method "GET" -Endpoint "/database/stats"

# 2. Create an Application
Write-Host "`n`n========== Application Management ==========" -ForegroundColor Cyan
$appData = @{
    app_name    = "Test Application"
    webhook_url = "https://example.com/webhook"
    settings    = @{
        rate_limit       = 1000
        retry_attempts   = 3
        default_template = ""
    }
}
$app = Invoke-APITest -Name "Create Application" -Method "POST" -Endpoint "/v1/apps" -Body $appData

if ($app) {
    $appId = $app.data.app_id
    $apiKey = $app.data.api_key
    Write-Host "`nSaved App ID: $appId" -ForegroundColor Green
    Write-Host "Saved API Key: $apiKey" -ForegroundColor Green
    
    # 3. Get Application Details
    Invoke-APITest -Name "Get Application" -Method "GET" -Endpoint "/v1/apps/$appId"
    
    # 4. List Applications
    Invoke-APITest -Name "List Applications" -Method "GET" -Endpoint "/v1/apps?page=1&page_size=10"
    
    # 5. Update Application
    $updateApp = @{
        app_name    = "Updated Test Application"
        webhook_url = "https://example.com/webhook-v2"
    }
    Invoke-APITest -Name "Update Application" -Method "PUT" -Endpoint "/v1/apps/$appId" -Body $updateApp
    
    # 6. Get Application Settings
    Invoke-APITest -Name "Get Application Settings" -Method "GET" -Endpoint "/v1/apps/$appId/settings"
    
    # 7. Update Application Settings
    $settingsUpdate = @{
        rate_limit       = 2000
        retry_attempts   = 5
        default_template = "welcome_template"
    }
    Invoke-APITest -Name "Update Application Settings" -Method "PUT" -Endpoint "/v1/apps/$appId/settings" -Body $settingsUpdate
    
    # Now test User Management with API Key Authentication
    Write-Host "`n`n========== User Management (Protected) ==========" -ForegroundColor Cyan
    $authHeaders = @{
        "Authorization" = "Bearer $apiKey"
    }
    
    # 8. Create a User
    $userData = @{
        external_user_id = "user123"
        email            = "user@example.com"
        phone            = "+1234567890"
        timezone         = "America/New_York"
        language         = "en"
        preferences      = @{
            email_enabled = $true
            push_enabled  = $true
            sms_enabled   = $false
            quiet_hours   = @{
                start = "22:00"
                end   = "08:00"
            }
        }
    }
    $user = Invoke-APITest -Name "Create User" -Method "POST" -Endpoint "/v1/users" -Body $userData -Headers $authHeaders
    
    if ($user) {
        $userId = $user.data.user_id
        Write-Host "`nSaved User ID: $userId" -ForegroundColor Green
        
        # 9. Get User Details
        Invoke-APITest -Name "Get User" -Method "GET" -Endpoint "/v1/users/$userId" -Headers $authHeaders
        
        # 10. List Users
        Invoke-APITest -Name "List Users" -Method "GET" -Endpoint "/v1/users?page=1&page_size=10" -Headers $authHeaders
        
        # 11. Update User
        $updateUser = @{
            email    = "updated@example.com"
            timezone = "America/Los_Angeles"
        }
        Invoke-APITest -Name "Update User" -Method "PUT" -Endpoint "/v1/users/$userId" -Body $updateUser -Headers $authHeaders
        
        # 12. Device Management
        Write-Host "`n`n========== Device Management ==========" -ForegroundColor Cyan
        $deviceData = @{
            platform = "ios"
            token    = "fcm-device-token-12345678"
        }
        Invoke-APITest -Name "Add Device" -Method "POST" -Endpoint "/v1/users/$userId/devices" -Body $deviceData -Headers $authHeaders
        
        # 13. Get User Devices
        $devices = Invoke-APITest -Name "Get User Devices" -Method "GET" -Endpoint "/v1/users/$userId/devices" -Headers $authHeaders
        
        if ($devices -and $devices.data.Count -gt 0) {
            $deviceId = $devices.data[0].device_id
            Write-Host "`nDevice ID: $deviceId" -ForegroundColor Green
            
            # 14. Remove Device
            Invoke-APITest -Name "Remove Device" -Method "DELETE" -Endpoint "/v1/users/$userId/devices/$deviceId" -Headers $authHeaders
        }
        
        # 15. Preferences Management
        Write-Host "`n`n========== Preferences Management ==========" -ForegroundColor Cyan
        Invoke-APITest -Name "Get User Preferences" -Method "GET" -Endpoint "/v1/users/$userId/preferences" -Headers $authHeaders
        
        $prefsUpdate = @{
            email_enabled = $false
            push_enabled  = $true
            sms_enabled   = $true
            quiet_hours   = @{
                start = "23:00"
                end   = "07:00"
            }
        }
        Invoke-APITest -Name "Update User Preferences" -Method "PUT" -Endpoint "/v1/users/$userId/preferences" -Body $prefsUpdate -Headers $authHeaders
        
        # 16. Delete User
        Write-Host "`n`n========== Cleanup ==========" -ForegroundColor Cyan
        Invoke-APITest -Name "Delete User" -Method "DELETE" -Endpoint "/v1/users/$userId" -Headers $authHeaders
    }
    
    # 17. Regenerate API Key
    Write-Host "`n`n========== API Key Management ==========" -ForegroundColor Cyan
    $newKey = Invoke-APITest -Name "Regenerate API Key" -Method "POST" -Endpoint "/v1/apps/$appId/regenerate-key"
    
    if ($newKey) {
        Write-Host "`nNew API Key: $($newKey.data.api_key)" -ForegroundColor Green
    }
    
    # 18. Delete Application
    Invoke-APITest -Name "Delete Application" -Method "DELETE" -Endpoint "/v1/apps/$appId"
}

# Test Error Handling
Write-Host "`n`n========== Error Handling Tests ==========" -ForegroundColor Cyan

# Test without API key
Write-Host "`n--- Testing Protected Endpoint Without API Key ---" -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "$baseUrl/v1/users" -Method GET -ContentType "application/json"
}
catch {
    Write-Host "Expected Error:" -ForegroundColor Green
    Write-Host $_.ErrorDetails.Message -ForegroundColor White
}

# Test with invalid API key
Write-Host "`n--- Testing Protected Endpoint With Invalid API Key ---" -ForegroundColor Yellow
try {
    Invoke-RestMethod -Uri "$baseUrl/v1/users" -Method GET -ContentType "application/json" -Headers @{Authorization = "Bearer invalid_key" }
}
catch {
    Write-Host "Expected Error:" -ForegroundColor Green
    Write-Host $_.ErrorDetails.Message -ForegroundColor White
}

# Test validation error
Write-Host "`n--- Testing Validation Error ---" -ForegroundColor Yellow
try {
    $invalidApp = @{
        app_name = "AB"  # Too short
    }
    Invoke-RestMethod -Uri "$baseUrl/v1/apps" -Method POST -ContentType "application/json" -Body ($invalidApp | ConvertTo-Json)
}
catch {
    Write-Host "Expected Error:" -ForegroundColor Green
    Write-Host $_.ErrorDetails.Message -ForegroundColor White
}

Write-Host "`n`n========================================" -ForegroundColor Cyan
Write-Host "API Testing Complete!" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan
