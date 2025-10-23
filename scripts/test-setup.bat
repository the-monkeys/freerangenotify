@echo off
echo Testing FreeRangeNotify setup...

echo 1. Testing configuration loading...
start /B go run cmd/server/main.go
timeout /T 3 /NOBREAK > nul

echo 2. Testing health endpoint...
powershell -Command "try { Invoke-RestMethod -Uri 'http://localhost:8080/health' } catch { Write-Host 'Health check failed' }"

echo 3. Testing version endpoint...
powershell -Command "try { Invoke-RestMethod -Uri 'http://localhost:8080/version' } catch { Write-Host 'Version check failed' }"

echo 4. Testing API status...
powershell -Command "try { Invoke-RestMethod -Uri 'http://localhost:8080/api/v1/status' } catch { Write-Host 'API status check failed' }"

echo 5. Stopping server...
taskkill /F /IM go.exe > nul 2>&1

echo ✅ Week 1 setup testing completed!