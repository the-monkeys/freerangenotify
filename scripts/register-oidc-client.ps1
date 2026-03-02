#!/usr/bin/env pwsh
# register-oidc-client.ps1
#
# Registers FreeRangeNotify as an OIDC client in Monkeys Identity.
# After running this script, copy the returned client_id and client_secret
# into your .env file as FREERANGE_OIDC_CLIENT_ID and FREERANGE_OIDC_CLIENT_SECRET.
#
# Prerequisites:
#   - Monkeys Identity must be running at $IdentityURL
#   - You must have admin credentials for Monkeys Identity

param(
    [string]$IdentityURL = "https://identity.monkeys.support",
    [string]$Email = "test@monkeys.com",
    [string]$Password = "Megamind@1",
    [string]$CallbackURL = "http://localhost:8080/v1/auth/sso/callback"
)

$ErrorActionPreference = "Stop"

Write-Host "`n=== Registering FreeRangeNotify OIDC Client ===" -ForegroundColor Cyan
Write-Host "Identity Provider: $IdentityURL"
Write-Host ""

# Step 1: Login to get admin token
Write-Host "[1/3] Logging in to Monkeys Identity as $Email..." -ForegroundColor Yellow
try {
    $loginBody = @{
        email    = $Email
        password = $Password
    } | ConvertTo-Json

    $loginResp = Invoke-RestMethod -Uri "$IdentityURL/api/v1/auth/login" `
        -Method Post `
        -ContentType "application/json" `
        -Body $loginBody

    $token = $loginResp.token
    if (-not $token) {
        # Try alternative response shapes
        $token = $loginResp.data.token
        if (-not $token) {
            $token = $loginResp.access_token
        }
        if (-not $token) {
            $token = $loginResp.data.access_token
        }
    }

    if (-not $token) {
        Write-Host "ERROR: Login succeeded but no token found in response." -ForegroundColor Red
        Write-Host "Response: $($loginResp | ConvertTo-Json -Depth 5)"
        exit 1
    }

    Write-Host "  Login successful." -ForegroundColor Green
}
catch {
    Write-Host "ERROR: Failed to login to Monkeys Identity." -ForegroundColor Red
    Write-Host "  $_"
    Write-Host ""
    Write-Host "Make sure Monkeys Identity is running and the credentials are correct."
    exit 1
}

# Step 2: Register OIDC client
Write-Host "[2/3] Registering OIDC client 'FreeRangeNotify'..." -ForegroundColor Yellow
try {
    $clientBody = @{
        client_name   = "FreeRangeNotify"
        redirect_uris = @($CallbackURL)
        scope         = "openid profile email"
        is_public     = $false
    } | ConvertTo-Json

    $clientResp = Invoke-RestMethod -Uri "$IdentityURL/api/v1/oauth2/clients" `
        -Method Post `
        -ContentType "application/json" `
        -Headers @{ Authorization = "Bearer $token" } `
        -Body $clientBody

    $clientData = $clientResp.data
    if (-not $clientData) {
        $clientData = $clientResp
    }

    $clientId = $clientData.client_id
    $clientSecret = $clientData.client_secret

    if (-not $clientId -or -not $clientSecret) {
        Write-Host "ERROR: Client created but missing client_id or client_secret in response." -ForegroundColor Red
        Write-Host "Response: $($clientResp | ConvertTo-Json -Depth 5)"
        exit 1
    }

    Write-Host "  Client registered successfully." -ForegroundColor Green
}
catch {
    Write-Host "ERROR: Failed to register OIDC client." -ForegroundColor Red
    Write-Host "  $_"
    exit 1
}

# Step 3: Output credentials
Write-Host ""
Write-Host "[3/3] OIDC Client Credentials:" -ForegroundColor Yellow
Write-Host "============================================" -ForegroundColor DarkGray
Write-Host "  Client ID:     $clientId" -ForegroundColor White
Write-Host "  Client Secret: $clientSecret" -ForegroundColor White
Write-Host "============================================" -ForegroundColor DarkGray
Write-Host ""
Write-Host "Add these to your .env file:" -ForegroundColor Cyan
Write-Host "  FREERANGE_OIDC_CLIENT_ID=$clientId"
Write-Host "  FREERANGE_OIDC_CLIENT_SECRET=$clientSecret"
Write-Host ""

# Step 4: Offer to auto-update .env
$envFile = Join-Path (Join-Path $PSScriptRoot "..") ".env"
if (Test-Path $envFile) {
    $answer = Read-Host "Update .env automatically? (y/n)"
    if ($answer -eq "y") {
        $content = Get-Content $envFile -Raw

        $content = $content -replace "(?m)^FREERANGE_OIDC_CLIENT_ID=.*$", "FREERANGE_OIDC_CLIENT_ID=$clientId"
        $content = $content -replace "(?m)^FREERANGE_OIDC_CLIENT_SECRET=.*$", "FREERANGE_OIDC_CLIENT_SECRET=$clientSecret"

        Set-Content $envFile -Value $content.TrimEnd() -NoNewline
        Write-Host ".env updated successfully." -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "IMPORTANT: Save the client_secret now - it cannot be retrieved later!" -ForegroundColor Red
Write-Host "Done." -ForegroundColor Green
