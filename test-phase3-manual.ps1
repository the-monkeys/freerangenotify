$baseUrl = "http://localhost:8080/v1"

# Helper function to handle errors
function Invoke-Api {
    param(
        [string]$Method,
        [string]$Uri,
        [hashtable]$Body
    )
    try {
        if ($Body) {
            $json = $Body | ConvertTo-Json -Depth 10
            Invoke-RestMethod -Method $Method -Uri $Uri -Body $json -ContentType "application/json" -Headers $headers
        }
        else {
            Invoke-RestMethod -Method $Method -Uri $Uri -ContentType "application/json" -Headers $headers
        }
    }
    catch {
        Write-Host "Error calling $Uri : $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader $_.Exception.Response.GetResponseStream()
            $reader.ReadToEnd()
        }
        return $null
    }
}

Write-Host "--- Starting Phase 3 Manual Tests ---" -ForegroundColor Cyan

# 1. Create Application
Write-Host "`n1. Creating Application..." -ForegroundColor Yellow
$appBody = @{
    app_name = "Phase3TestApp"
    settings = @{
        rate_limit       = 100
        retry_attempts   = 3
        default_template = "default"
    }
}

# Initial call without headers
try {
    $json = $appBody | ConvertTo-Json -Depth 10
    $app = Invoke-RestMethod -Method POST -Uri "$baseUrl/apps" -Body $json -ContentType "application/json"
}
catch {
    Write-Host "Error creating app: $($_.Exception.Message)" -ForegroundColor Red
    exit
}

if (-not $app) { exit }
$appId = $app.data.app_id
$apiKey = $app.data.api_key
Write-Host "App Created: ID=$appId, Key=$apiKey" -ForegroundColor Green

# Add Authorization header for subsequent requests
$headers = @{
    "Authorization" = "Bearer $apiKey"
}

# 2. Create User
Write-Host "`n2. Creating User..." -ForegroundColor Yellow
$userId = "user_phase3_$(Get-Random)"
$userBody = @{
    external_user_id = $userId
    email            = "$userId@example.com"
    preferences      = @{
        email_enabled = $true
        push_enabled  = $true
        dnd           = $false
        daily_limit   = 100
    }
}
$user = Invoke-Api -Method POST -Uri "$baseUrl/users" -Body $userBody
if (-not $user) { exit }
$realUserId = $user.data.user_id
Write-Host "User Created: ID=$realUserId" -ForegroundColor Green

# 3. Test Scheduled Notification
Write-Host "`n3. Testing Scheduled Notification..." -ForegroundColor Yellow
$scheduledTime = (Get-Date).AddSeconds(5).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$notifBody = @{
    user_id      = $realUserId
    channel      = "email"
    template_id  = "default"
    priority     = "normal"
    scheduled_at = $scheduledTime
    title        = "Scheduled Test"
    body         = "This should be delivered later"
}
$notif = Invoke-Api -Method POST -Uri "$baseUrl/notifications" -Body $notifBody
if ($notif) {
    # Handle array response (bulk send) or single object
    if ($notif -is [array]) { $notif = $notif[0] }
    
    $notifId = $notif.notification_id
    Write-Host "Notification Scheduled: ID=$notifId, Time=$scheduledTime" -ForegroundColor Green
    
    # Check status immediately
    $status = (Invoke-Api -Method GET -Uri "$baseUrl/notifications/$notifId").status
    Write-Host "Immediate Status: $status" -ForegroundColor Cyan
    
    Write-Host "Waiting 10 seconds..."
    Start-Sleep -Seconds 10
    
    # Check status after wait
    $status = (Invoke-Api -Method GET -Uri "$baseUrl/notifications/$notifId").status
    Write-Host "Final Status: $status" -ForegroundColor Cyan
}

# 4. Test DND
Write-Host "`n4. Testing DND..." -ForegroundColor Yellow
# Enable DND
$prefBody = @{
    preferences = @{
        dnd           = $true
        email_enabled = $true
        push_enabled  = $true
    }
}
$updatedUser = Invoke-Api -Method PUT -Uri "$baseUrl/users/$realUserId" -Body $prefBody
Write-Host "DND Enabled: $($updatedUser.preferences.dnd)" -ForegroundColor Green

# Send Normal Notification (Should Fail)
Write-Host "Sending Normal Priority Notification (Should Fail)..."
$dndNotifBody = @{
    user_id     = $realUserId
    channel     = "email"
    template_id = "default"
    priority    = "normal"
    title       = "DND Test"
    body        = "Should be blocked"
}
# Expecting error here, so we handle it
try {
    $json = $dndNotifBody | ConvertTo-Json -Depth 10
    Invoke-RestMethod -Method POST -Uri "$baseUrl/notifications" -Body $json -ContentType "application/json" -Headers $headers
    Write-Host "FAILED: Notification should have been blocked!" -ForegroundColor Red
}
catch {
    Write-Host "SUCCESS: Notification blocked as expected. Error: $($_.Exception.Message)" -ForegroundColor Green
}

# Send Critical Notification (Should Pass)
Write-Host "Sending Critical Priority Notification (Should Pass)..."
$critNotifBody = @{
    user_id     = $realUserId
    channel     = "email"
    template_id = "default"
    priority    = "critical"
    title       = "Critical Test"
    body        = "Should bypass DND"
}
$critNotif = Invoke-Api -Method POST -Uri "$baseUrl/notifications" -Body $critNotifBody
if ($critNotif) {
    if ($critNotif -is [array]) { $critNotif = $critNotif[0] }
    Write-Host "SUCCESS: Critical notification sent. ID=$($critNotif.notification_id)" -ForegroundColor Green
}

# 5. Test Daily Limit
Write-Host "`n5. Testing Daily Limit..." -ForegroundColor Yellow
# Set limit to 1
$limitBody = @{
    preferences = @{
        dnd           = $false
        daily_limit   = 1
        email_enabled = $true
        push_enabled  = $true
    }
}
Invoke-Api -Method PUT -Uri "$baseUrl/users/$realUserId" -Body $limitBody | Out-Null
Write-Host "Daily Limit set to 1" -ForegroundColor Green

# Send 1st (Should Pass)
Write-Host "Sending 1st Notification..."
$limitNotifBody = @{
    user_id     = $realUserId
    channel     = "email"
    template_id = "default"
    priority    = "normal"
    title       = "Limit Test 1"
    body        = "Should pass"
}
$n1 = Invoke-Api -Method POST -Uri "$baseUrl/notifications" -Body $limitNotifBody
if ($n1) { Write-Host "1st Passed" -ForegroundColor Green }

# Send 2nd (Should Fail)
Write-Host "Sending 2nd Notification (Should Fail)..."
try {
    $json = $limitNotifBody | ConvertTo-Json -Depth 10
    Invoke-RestMethod -Method POST -Uri "$baseUrl/notifications" -Body $json -ContentType "application/json" -Headers $headers
    Write-Host "FAILED: Notification should have been blocked by limit!" -ForegroundColor Red
}
catch {
    Write-Host "SUCCESS: Notification blocked by limit. Error: $($_.Exception.Message)" -ForegroundColor Green
}

Write-Host "`n--- Test Complete ---" -ForegroundColor Cyan
