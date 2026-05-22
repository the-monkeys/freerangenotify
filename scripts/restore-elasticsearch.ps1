<#
.SYNOPSIS
    Restore the local ./elasticsearch snapshot into the freerange-elasticsearch
    container using docker compose.

.DESCRIPTION
    1. Ensures Elasticsearch is running with ./elasticsearch mounted as a
       snapshot repository (via docker-compose.restore.yml override).
    2. Waits for the cluster to be healthy.
    3. Registers the filesystem snapshot repository at /usr/share/elasticsearch/snapshots.
    4. Resolves the latest snapshot in that repo.
    5. Closes any existing indices that would conflict, then restores.

.PARAMETER SnapshotName
    Explicit snapshot name to restore. Defaults to the most recent snapshot
    found in the repository.

.PARAMETER RepoName
    Snapshot repository name to register in ES. Default: 'local_backup'.

.PARAMETER IndicesPattern
    Comma-separated index pattern to restore. Default: '*' (all indices in
    the snapshot, excluding system indices).

.PARAMETER NoRecreate
    If set, skip 'docker compose down -v' and reuse the existing ES volume.
    Default behavior is non-destructive (no -v) — pass -Recreate to wipe.

.PARAMETER Recreate
    If set, run 'docker compose down -v' first to wipe existing ES data
    before restoring. DESTRUCTIVE.

.EXAMPLE
    .\scripts\restore-elasticsearch.ps1

.EXAMPLE
    .\scripts\restore-elasticsearch.ps1 -Recreate -SnapshotName snapshot_2025_11_15
#>
[CmdletBinding()]
param(
    [string]$SnapshotName,
    [string]$RepoName = 'local_backup',
    [string]$IndicesPattern = '*',
    [switch]$Recreate
)

$ErrorActionPreference = 'Stop'

# Resolve repo root (script lives in <repo>/scripts)
$RepoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $RepoRoot
try {
    $SnapshotDir = Join-Path $RepoRoot 'elasticsearch'
    if (-not (Test-Path $SnapshotDir)) {
        throw "Snapshot directory not found: $SnapshotDir"
    }
    if (-not (Test-Path (Join-Path $SnapshotDir 'index.latest'))) {
        Write-Warning "No 'index.latest' file under $SnapshotDir - this may not be a valid ES snapshot repo."
    }

    $ComposeFiles = @('-f', 'docker-compose.yml', '-f', 'docker-compose.restore.yml')
    $EsUrl = 'http://localhost:9200'
    $ContainerName = 'freerange-elasticsearch'

    function Invoke-Compose {
        param([Parameter(ValueFromRemainingArguments = $true)][string[]]$ComposeArgs)
        & docker compose @ComposeFiles @ComposeArgs
        if ($LASTEXITCODE -ne 0) {
            throw "docker compose $($ComposeArgs -join ' ') failed (exit $LASTEXITCODE)"
        }
    }

    function Invoke-Es {
        param(
            [Parameter(Mandatory)][ValidateSet('GET', 'PUT', 'POST', 'DELETE')][string]$Method,
            [Parameter(Mandatory)][string]$Path,
            [object]$Body
        )
        $params = @{
            Method      = $Method
            Uri         = "$EsUrl$Path"
            ContentType = 'application/json'
            ErrorAction = 'Stop'
        }
        if ($PSBoundParameters.ContainsKey('Body') -and $null -ne $Body) {
            $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress)
        }
        return Invoke-RestMethod @params
    }

    if ($Recreate) {
        Write-Host '==> Tearing down existing stack (volumes will be removed)...' -ForegroundColor Yellow
        Invoke-Compose down -v
    }

    Write-Host '==> Starting Elasticsearch with snapshot repo mounted...' -ForegroundColor Cyan
    Invoke-Compose up -d elasticsearch

    Write-Host '==> Waiting for cluster to become available...' -ForegroundColor Cyan
    $deadline = (Get-Date).AddMinutes(2)
    while ($true) {
        try {
            $health = Invoke-RestMethod -Method GET -Uri "$EsUrl/_cluster/health" -TimeoutSec 3 -ErrorAction Stop
            if ($health.status -in @('yellow', 'green')) {
                Write-Host "    cluster status: $($health.status)" -ForegroundColor Green
                break
            }
        }
        catch {
            # not ready yet
        }
        if ((Get-Date) -gt $deadline) {
            throw "Elasticsearch did not become healthy within 2 minutes."
        }
        Start-Sleep -Seconds 2
    }

    Write-Host "==> Registering snapshot repository '$RepoName' -> /usr/share/elasticsearch/snapshots" -ForegroundColor Cyan
    $repoBody = @{
        type     = 'fs'
        settings = @{
            location = '/usr/share/elasticsearch/snapshots'
            readonly = $true
        }
    }
    Invoke-Es -Method PUT -Path "/_snapshot/$RepoName" -Body $repoBody | Out-Null

    Write-Host "==> Verifying repository..." -ForegroundColor Cyan
    Invoke-Es -Method POST -Path "/_snapshot/$RepoName/_verify" | Out-Null

    Write-Host "==> Listing snapshots in '$RepoName'..." -ForegroundColor Cyan
    $listing = Invoke-Es -Method GET -Path "/_snapshot/$RepoName/_all"
    if (-not $listing.snapshots -or $listing.snapshots.Count -eq 0) {
        throw "No snapshots found in repository '$RepoName'."
    }

    $snaps = $listing.snapshots | Sort-Object { [datetime]$_.start_time }
    Write-Host "    Found $($snaps.Count) snapshot(s):" -ForegroundColor DarkGray
    foreach ($s in $snaps) {
        Write-Host "      - $($s.snapshot)  state=$($s.state)  start=$($s.start_time)" -ForegroundColor DarkGray
    }

    if (-not $SnapshotName) {
        $SnapshotName = ($snaps | Select-Object -Last 1).snapshot
        Write-Host "==> No -SnapshotName given; using latest: $SnapshotName" -ForegroundColor Yellow
    }

    $target = $snaps | Where-Object { $_.snapshot -eq $SnapshotName }
    if (-not $target) {
        throw "Snapshot '$SnapshotName' not found in '$RepoName'."
    }

    # Close conflicting indices so restore can replace them. Safer than deleting
    # blindly. Pattern is restricted to user indices (skip system indices `.*`).
    Write-Host "==> Closing existing user indices to allow restore..." -ForegroundColor Cyan
    try {
        $allIdx = Invoke-Es -Method GET -Path '/_cat/indices?h=index&format=json'
        $userIdx = @($allIdx | Where-Object { $_.index -notlike '.*' } | ForEach-Object { $_.index })
        if ($userIdx.Count -gt 0) {
            $closeTarget = ($userIdx -join ',')
            Write-Host "    closing: $closeTarget" -ForegroundColor DarkGray
            Invoke-Es -Method POST -Path "/$closeTarget/_close" | Out-Null
        }
        else {
            Write-Host "    (none to close)" -ForegroundColor DarkGray
        }
    }
    catch {
        Write-Warning "Failed to close existing indices: $($_.Exception.Message). Restore may fail if names conflict."
    }

    Write-Host "==> Restoring snapshot '$SnapshotName' (indices: $IndicesPattern)..." -ForegroundColor Cyan
    $restoreBody = @{
        indices              = $IndicesPattern
        include_global_state = $false
        ignore_unavailable   = $true
    }
    Invoke-Es -Method POST -Path "/_snapshot/$RepoName/$SnapshotName/_restore?wait_for_completion=true" -Body $restoreBody | Out-Null

    Write-Host "==> Waiting for cluster to settle..." -ForegroundColor Cyan
    Invoke-RestMethod -Method GET -Uri "$EsUrl/_cluster/health?wait_for_status=yellow&timeout=60s" | Out-Null

    Write-Host "==> Restored indices:" -ForegroundColor Green
    & docker exec $ContainerName curl -s "http://localhost:9200/_cat/indices?v"

    Write-Host "`nRestore complete." -ForegroundColor Green
    Write-Host "Next: bring up the rest of the stack with:" -ForegroundColor Yellow
    Write-Host "    docker compose up -d" -ForegroundColor Yellow
}
finally {
    Pop-Location
}
