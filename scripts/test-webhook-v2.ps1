param(
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$AdminToken = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($AdminToken)) {
    $AdminToken = $env:INTEGRATION_ADMIN_TOKEN
}
if ([string]::IsNullOrWhiteSpace($AdminToken)) {
    $AdminToken = $env:FREERANGE_ADMIN_TOKEN
}
if ([string]::IsNullOrWhiteSpace($AdminToken)) {
    throw "Admin token is required. Set -AdminToken or INTEGRATION_ADMIN_TOKEN."
}

$env:INTEGRATION_WEBHOOK_V2 = "true"
$env:INTEGRATION_BASE_URL = $BaseUrl
$env:INTEGRATION_ADMIN_TOKEN = $AdminToken

Write-Host "Running webhook-v2 integration suite against $BaseUrl" -ForegroundColor Cyan
go test -v ./tests/integration/webhook -count=1
if ($LASTEXITCODE -ne 0) {
    throw "Webhook-v2 integration suite failed."
}

Write-Host "Webhook-v2 integration suite passed." -ForegroundColor Green
