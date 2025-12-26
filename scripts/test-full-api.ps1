# FreeRangeNotify Full API Test Script using curl.exe
# Refactored to use temporary files for request bodies to avoid Windows shell quoting issues.

$baseUrl = "http://localhost:8080"
$outputFile = "full_api_test_results.txt"
$bodyFile = "req_body.txt"
$ErrorActionPreference = "Continue"

function Run-Test {
    param (
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [string]$Body = $null,
        [string]$AuthToken = $null,
        [string]$ExtraHeaders = $null
    )

    Write-Host "`n--- $Name ---" -ForegroundColor Cyan
    $header = "`n--- $Name ---`n"
    $header | Out-File -FilePath $outputFile -Append -Encoding UTF8
    
    $curlCmd = "curl.exe -s -X $Method `"$Url`""
    
    if ($AuthToken) {
        $curlCmd += " -H `"Authorization: Bearer $AuthToken`""
    }
    
    if ($ExtraHeaders) {
        $curlCmd += " $ExtraHeaders"
    }

    if ($Body) {
        # Write body to file with ASCII encoding to avoid UTF8 BOM issues with curl
        $Body | Out-File -FilePath $bodyFile -Encoding ASCII
        $curlCmd += " -H `"Content-Type: application/json`" -d `"@$bodyFile`""
        
        Write-Host "Body: $Body" -ForegroundColor DarkGray
        $cmdLog = "Body: $Body`n"
        $cmdLog | Out-File -FilePath $outputFile -Append -Encoding UTF8
    }

    Write-Host "CMD: $curlCmd" -ForegroundColor DarkGray
    $cmdLog = "CMD: $curlCmd`n"
    $cmdLog | Out-File -FilePath $outputFile -Append -Encoding UTF8

    try {
        # Run curl
        $output = Invoke-Expression "$curlCmd 2>&1"
        
        Write-Host "$output" -ForegroundColor Green
        $output | Out-File -FilePath $outputFile -Append -Encoding UTF8
        return $output
    } catch {
        Write-Host "Error executing command: $_" -ForegroundColor Red
        "Error: $_" | Out-File -FilePath $outputFile -Append -Encoding UTF8
    }
}

# Clear output
"" | Out-File -FilePath $outputFile -Encoding UTF8
if (Test-Path $bodyFile) { Remove-Item $bodyFile }

Write-Host "Starting Full API Test..." -ForegroundColor Yellow

# --- System ---
Run-Test -Name "Health Check" -Method "GET" -Url "$baseUrl/v1/health"

# --- Apps ---
# Create App
$createAppBody = '{"app_name": "FullTestApp", "webhook_url": "http://host.docker.internal:8090/webhook"}'
$appOutput = Run-Test -Name "Create App" -Method "POST" -Url "$baseUrl/v1/apps" -Body $createAppBody

# Parse App ID/Key
try {
    $parts = $appOutput -split "`n"
    # Find the JSON part - usually the last line or the one starting with {
    $jsonStr = $parts | Where-Object { $_.Trim().StartsWith("{") } | Select-Object -Last 1
    $appJson = $jsonStr | ConvertFrom-Json
    $appId = $appJson.data.app_id
    $apiKey = $appJson.data.api_key
    Write-Host "App ID: $appId" -ForegroundColor Magenta
    Write-Host "API Key: $apiKey" -ForegroundColor Magenta
} catch {
    Write-Host "Failed to parse App output: $_" -ForegroundColor Red
    $appId = $null
    $apiKey = $null
}

if (-not $appId) {
    Write-Host "Stopping tests due to App creation failure." -ForegroundColor Red
    exit
}

# Get App
Run-Test -Name "Get App" -Method "GET" -Url "$baseUrl/v1/apps/$appId"

# List Apps
Run-Test -Name "List Apps" -Method "GET" -Url "$baseUrl/v1/apps?page=1&page_size=10"

# Update App
$updateAppBody = '{"app_name": "FullTestApp Updated"}'
Run-Test -Name "Update App" -Method "PUT" -Url "$baseUrl/v1/apps/$appId" -Body $updateAppBody

# Get Settings
Run-Test -Name "Get Settings" -Method "GET" -Url "$baseUrl/v1/apps/$appId/settings"

# Update Settings
$updateSettingsBody = '{"rate_limit": 500, "retry_attempts": 5}'
Run-Test -Name "Update Settings" -Method "PUT" -Url "$baseUrl/v1/apps/$appId/settings" -Body $updateSettingsBody

# --- Users ---
# Create User
$createUserBody = '{"external_user_id": "u-test-01", "email": "testuser@example.com", "preferences": {"email_enabled": true}}'
$userOutput = Run-Test -Name "Create User" -Method "POST" -Url "$baseUrl/v1/users" -Body $createUserBody -AuthToken $apiKey

try {
    $parts = $userOutput -split "`n"
    $jsonStr = $parts | Where-Object { $_.Trim().StartsWith("{") } | Select-Object -Last 1
    $userJson = $jsonStr | ConvertFrom-Json
    $userId = $userJson.data.user_id
    Write-Host "User ID: $userId" -ForegroundColor Magenta
} catch {
    Write-Host "Failed to parse User output." -ForegroundColor Red
}

# Get User
Run-Test -Name "Get User" -Method "GET" -Url "$baseUrl/v1/users/$userId" -AuthToken $apiKey

# List Users
Run-Test -Name "List Users" -Method "GET" -Url "$baseUrl/v1/users?page=1&page_size=10" -AuthToken $apiKey

# Update User
$updateUserBody = '{"email": "updated@example.com"}'
Run-Test -Name "Update User" -Method "PUT" -Url "$baseUrl/v1/users/$userId" -Body $updateUserBody -AuthToken $apiKey

# Add Device
$addDeviceBody = '{"platform": "ios", "token": "device-token-123", "active": true}'
Run-Test -Name "Add Device" -Method "POST" -Url "$baseUrl/v1/users/$userId/devices" -Body $addDeviceBody -AuthToken $apiKey

# Get Devices
Run-Test -Name "Get Devices" -Method "GET" -Url "$baseUrl/v1/users/$userId/devices" -AuthToken $apiKey

# Get Preferences
Run-Test -Name "Get Preferences" -Method "GET" -Url "$baseUrl/v1/users/$userId/preferences" -AuthToken $apiKey

# Update Preferences
$updatePrefsBody = '{"email_enabled": true, "push_enabled": true}'
Run-Test -Name "Update Preferences" -Method "PUT" -Url "$baseUrl/v1/users/$userId/preferences" -Body $updatePrefsBody -AuthToken $apiKey

# --- Templates ---
# Create Template
$createTemplateBody = '{"app_id": "' + $appId + '", "name": "welcome_template", "channel": "email", "subject": "Welcome!", "body": "Hello {{.name}}, welcome to our service.", "variables": ["name"]}'
$tplOutput = Run-Test -Name "Create Template" -Method "POST" -Url "$baseUrl/v1/templates" -Body $createTemplateBody -AuthToken $apiKey

try {
    $parts = $tplOutput -split "`n"
    # Find the JSON line - assuming it's the one starting with {
    $jsonStr = $parts | Where-Object { $_.Trim().StartsWith("{") } | Select-Object -Last 1
    $tplJson = $jsonStr | ConvertFrom-Json
    
    # Template response is direct object, not wrapped in data
    if ($tplJson.id) {
        $templateId = $tplJson.id
    } elseif ($tplJson.data.id) {
        $templateId = $tplJson.data.id
    } else {
        $templateId = $tplJson.data.template_id
    }

    Write-Host "Template ID: $templateId" -ForegroundColor Magenta
} catch {
    Write-Host "Failed to parse Template output." -ForegroundColor Red
}

# List Templates
Run-Test -Name "List Templates" -Method "GET" -Url "$baseUrl/v1/templates?page=1&page_size=10" -AuthToken $apiKey

# Get Template
Run-Test -Name "Get Template" -Method "GET" -Url "$baseUrl/v1/templates/$templateId" -AuthToken $apiKey

# Update Template
$updateTemplateBody = '{"body": "Hello {{.name}}, welcome back!"}'
Run-Test -Name "Update Template" -Method "PUT" -Url "$baseUrl/v1/templates/$templateId" -Body $updateTemplateBody -AuthToken $apiKey

# Render Template
$renderTemplateBody = '{"data": {"name": "John Doe"}}'
Run-Test -Name "Render Template" -Method "POST" -Url "$baseUrl/v1/templates/$templateId/render" -Body $renderTemplateBody -AuthToken $apiKey

# --- Notifications ---
# Send Notification
$sendNotifBody = '{"user_id": "' + $userId + '", "template_id": "' + $templateId + '", "channel": "email", "priority": "high", "title": "Default Title", "body": "Default Body", "data": {"name": "John"}}'
$notifOutput = Run-Test -Name "Send Notification" -Method "POST" -Url "$baseUrl/v1/notifications" -Body $sendNotifBody -AuthToken $apiKey

try {
    $parts = $notifOutput -split "`n"
    $jsonStr = $parts | Where-Object { $_.Trim().StartsWith("{") } | Select-Object -Last 1
    $notifJson = $jsonStr | ConvertFrom-Json
    
    if ($notifJson.notification_id) {
        $notifId = $notifJson.notification_id
    } elseif ($notifJson.data.notification_id) {
        $notifId = $notifJson.data.notification_id
    }
    
    Write-Host "Notification ID: $notifId" -ForegroundColor Magenta
} catch {
    Write-Host "Failed to parse Notification output." -ForegroundColor Red
}

# List Notifications
Run-Test -Name "List Notifications" -Method "GET" -Url "$baseUrl/v1/notifications?page=1&page_size=10" -AuthToken $apiKey

# Get Notification
Run-Test -Name "Get Notification" -Method "GET" -Url "$baseUrl/v1/notifications/$notifId" -AuthToken $apiKey

# Update Notification Status
$updateStatusBody = '{"status": "read"}'
Run-Test -Name "Update Notification Status" -Method "PUT" -Url "$baseUrl/v1/notifications/$notifId/status" -Body $updateStatusBody -AuthToken $apiKey

# Retry Notification
Run-Test -Name "Retry Notification" -Method "POST" -Url "$baseUrl/v1/notifications/$notifId/retry" -AuthToken $apiKey

# Cancel Notification
Run-Test -Name "Cancel Notification" -Method "DELETE" -Url "$baseUrl/v1/notifications/$notifId" -AuthToken $apiKey

# --- Admin ---
# Queue Stats
Run-Test -Name "Queue Stats" -Method "GET" -Url "$baseUrl/v1/admin/queues/stats" -AuthToken $apiKey

# DLQ List
Run-Test -Name "DLQ List" -Method "GET" -Url "$baseUrl/v1/admin/queues/dlq" -AuthToken $apiKey

# --- Cleanup ---
Run-Test -Name "Delete User" -Method "DELETE" -Url "$baseUrl/v1/users/$userId" -AuthToken $apiKey
Run-Test -Name "Delete Template" -Method "DELETE" -Url "$baseUrl/v1/templates/$templateId" -AuthToken $apiKey
Run-Test -Name "Delete App" -Method "DELETE" -Url "$baseUrl/v1/apps/$appId" # Try without auth, it's public in routes.go

Write-Host "`nRefactored Test Execution Complete." -ForegroundColor Yellow
if (Test-Path $bodyFile) { Remove-Item $bodyFile }
