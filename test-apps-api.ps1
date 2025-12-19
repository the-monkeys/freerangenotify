# Test Script for /v1/apps APIs

$BaseUrl = "http://localhost:8080/v1/apps"
$TempFile = ".\temp_payload.json"

Write-Host "1. Testing Create Application..."
# Corrected field name: app_name instead of name
$CreatePayload = '{"app_name": "Curl Test App", "settings": {"rate_limit": 100}}'
$CreatePayload | Out-File -Encoding ASCII $TempFile
$CreateResponse = curl.exe -s -X POST $BaseUrl -H "Content-Type: application/json" -d "@$TempFile"
Write-Host "Response: $CreateResponse"

# Extract ID - The response structure might also be different (ApplicationResponse has app_id)
# Response ID field is app_id
if ($CreateResponse -match '"app_id":"([^"]+)"') {
    $AppId = $matches[1]
    Write-Host "Created App ID: $AppId"
}
else {
    Write-Host "Regex failed on: $CreateResponse"
    Write-Error "Failed to extract App ID. Stopping."
    Remove-Item $TempFile -ErrorAction SilentlyContinue
    exit 1
}

Write-Host "`n2. Testing List Applications..."
curl.exe -s -X GET "$BaseUrl/"
Write-Host ""

Write-Host "`n3. Testing Get Application By ID..."
curl.exe -s -X GET "$BaseUrl/$AppId"
Write-Host ""

Write-Host "`n4. Testing Update Application..."
$UpdatePayload = '{"app_name": "Curl Test App Updated"}'
$UpdatePayload | Out-File -Encoding ASCII $TempFile
curl.exe -s -X PUT "$BaseUrl/$AppId" -H "Content-Type: application/json" -d "@$TempFile"
Write-Host ""

Write-Host "`n5. Testing Update Settings..."
$SettingsPayload = '{"rate_limit": 1000, "retry_attempts": 3}'
$SettingsPayload | Out-File -Encoding ASCII $TempFile
curl.exe -s -X PUT "$BaseUrl/$AppId/settings" -H "Content-Type: application/json" -d "@$TempFile"
Write-Host ""

Write-Host "`n6. Testing Get Settings..."
curl.exe -s -X GET "$BaseUrl/$AppId/settings"
Write-Host ""

Write-Host "`n7. Testing Regenerate API Key..."
# Empty JSON object
'{}' | Out-File -Encoding ASCII $TempFile
curl.exe -s -X POST "$BaseUrl/$AppId/regenerate-key" -H "Content-Type: application/json" -d "@$TempFile"
Write-Host ""

Write-Host "`n8. Testing Delete Application..."
curl.exe -s -X DELETE "$BaseUrl/$AppId"
Write-Host "Delete request sent."

Write-Host "`n9. Verify Deletion (Should be 404 or empty)..."
curl.exe -s -X GET "$BaseUrl/$AppId"
Write-Host ""

# Cleanup
Remove-Item $TempFile -ErrorAction SilentlyContinue
