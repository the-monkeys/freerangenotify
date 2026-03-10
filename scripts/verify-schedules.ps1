# Verify scheduled notifications and workflow schedules from Elasticsearch
# Usage: .\scripts\verify-schedules.ps1
# Requires: Elasticsearch at http://localhost:9200

$ES = "http://localhost:9200"

Write-Host "=== Elasticsearch: $ES ===`n"

# 1. Notifications with scheduled_at
Write-Host "--- Notifications with scheduled_at (from Notification tab) ---"
$notifs = Invoke-RestMethod -Uri "$ES/notifications/_search" -Method Post -ContentType "application/json" -Body '{"query":{"exists":{"field":"scheduled_at"}},"sort":[{"scheduled_at":"desc"}],"size":50,"_source":["notification_id","user_id","channel","status","scheduled_at","sent_at","created_at","title"]}'
$notifs.hits.hits | ForEach-Object {
    $s = $_._source
    Write-Host "ID: $($s.notification_id.Substring(0,8))... | scheduled: $($s.scheduled_at) | sent: $($s.sent_at) | status: $($s.status)"
}
Write-Host "Total: $($notifs.hits.total.value) scheduled notification(s)`n"

# 2. Workflow schedules (cron)
Write-Host "--- Workflow Schedules (Cron jobs) ---"
$scheds = Invoke-RestMethod -Uri "$ES/workflow_schedules/_search" -Method Post -ContentType "application/json" -Body '{"query":{"match_all":{}},"_source":["id","name","app_id","workflow_trigger_id","cron","target_type","status","last_run_at","created_at"]}'
$scheds.hits.hits | ForEach-Object {
    $s = $_._source
    Write-Host "Name: $($s.name) | cron: $($s.cron) | status: $($s.status) | last_run: $($s.last_run_at)"
}
Write-Host "Total: $($scheds.hits.total.value) schedule(s)`n"
