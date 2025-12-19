$API_BASE = "http://localhost:8080/v1"
$API_KEY = "frn_LWn4_v-1PEEFJLaWgA9sT-Wq3NBon0px8v_sLF545Ks="
$APP_ID = "99603bd3-d9f8-40ed-82bf-88e4646e081e"

function Test-ScopedAPIs {
    Write-Host "--- Testing Scoped APIs ---" -ForegroundColor Cyan

    # 1. Create User
    Write-Host "`n[1] Creating User..." -ForegroundColor Yellow
    $userBody = @{
        external_user_id = "user_123"
        email            = "test@example.com"
        timezone         = "UTC"
        language         = "en"
    } | ConvertTo-Json
    $userResp = Invoke-RestMethod -Uri "$API_BASE/users/" -Method Post -Body $userBody -Headers @{"Authorization" = $API_KEY } -ContentType "application/json"
    $userID = $userResp.data.user_id
    Write-Host "User Created ID: $userID" -ForegroundColor Green

    # 2. List Users
    Write-Host "`n[2] Listing Users..." -ForegroundColor Yellow
    $listUsers = Invoke-RestMethod -Uri "$API_BASE/users/" -Method Get -Headers @{"Authorization" = $API_KEY }
    Write-Host "Found $($listUsers.data.users.Count) users" -ForegroundColor Green

    # 3. Create Template
    Write-Host "`n[3] Creating Template..." -ForegroundColor Yellow
    $templateBody = @{
        app_id      = $APP_ID
        name        = "welcome_email_" + (Get-Date -Format "yyyyMMddHHmmss")
        channel     = "email"
        subject     = "Welcome to our app!"
        body        = "Hello {{.name}}, welcome aboard!"
        variables   = @("name")
        description = "Initial welcome email"
    } | ConvertTo-Json
    $templateResp = Invoke-RestMethod -Uri "$API_BASE/templates/" -Method Post -Body $templateBody -Headers @{"Authorization" = $API_KEY } -ContentType "application/json"
    Write-Host "DEBUG Template Resp: $($templateResp | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
    $templateID = $templateResp.id
    Write-Host "Template Created ID: $templateID" -ForegroundColor Green

    # 4. Send Notification
    Write-Host "`n[4] Sending Notification..." -ForegroundColor Yellow
    $notifBody = @{
        user_id     = $userID
        template_id = $templateID
        data        = @{ name = "John Doe" }
        channel     = "email"
        priority    = "normal"
        title       = "Welcome!"
        body        = "Placeholder body (template will override)"
    } | ConvertTo-Json
    $notifResp = Invoke-RestMethod -Uri "$API_BASE/notifications/" -Method Post -Body $notifBody -Headers @{"Authorization" = $API_KEY } -ContentType "application/json"
    Write-Host "DEBUG Notification Resp: $($notifResp | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
    $notificationID = $notifResp.notification_id
    Write-Host "Notification Sent ID: $notificationID" -ForegroundColor Green

    # 5. List Notifications
    Write-Host "`n[5] Listing Notifications..." -ForegroundColor Yellow
    Start-Sleep -Seconds 2 # Wait for ES index
    $listNotifs = Invoke-RestMethod -Uri "$API_BASE/notifications/" -Method Get -Headers @{"Authorization" = $API_KEY }
    Write-Host "Found $($listNotifs.notifications.Count) notifications" -ForegroundColor Green

    Write-Host "`n--- Verification Complete ---" -ForegroundColor Cyan
}

Test-ScopedAPIs
