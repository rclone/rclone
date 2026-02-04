# --- CONFIGURATION ---
param(
    [string]$RemoteName = "mysharepoint"
)
# ---------------------

# 1. Get the Access Token from rclone config
Write-Host "Getting token for remote '$RemoteName'..." -ForegroundColor Cyan

try {
    # Run rclone config dump and parse JSON
    $ConfigJson = rclone config dump | ConvertFrom-Json
    
    # Check if remote exists
    if (-not $ConfigJson.$RemoteName) {
        Write-Error "Remote '$RemoteName' not found in rclone config."
        exit 1
    }

    # Extract the token string (which is itself a JSON string)
    $TokenData = $ConfigJson.$RemoteName.token | ConvertFrom-Json
    $AccessToken = $TokenData.access_token

    if ([string]::IsNullOrWhiteSpace($AccessToken)) {
        Write-Error "No access token found. Run 'rclone about ${RemoteName}:' to refresh it."
        exit 1
    }
}
catch {
    Write-Error "Failed to retrieve token from rclone. Error: $_"
    exit 1
}

# 2. Setup Headers for Graph API
$Headers = @{
    "Authorization" = "Bearer $AccessToken"
    "Content-Type"  = "application/json"
}

Write-Host "Searching for sites..." -ForegroundColor Cyan
Write-Host ""
Write-Host ("{0,-35} | {1,-20} | {2,-20} | {3,-40} | {4}" -f "SITE NAME", "DRIVE NAME", "DRIVE ID", "SITE URL", "SITE ID")
Write-Host ("-" * 160)

# 3. Query All Sites
$SitesUrl = "https://graph.microsoft.com/v1.0/sites?search=*"

try {
    $SitesResponse = Invoke-RestMethod -Uri $SitesUrl -Headers $Headers -Method Get
    $Sites = $SitesResponse.value

    if (-not $Sites) {
        Write-Warning "No sites found."
        exit
    }

    foreach ($Site in $Sites) {
        $SiteId = $Site.id
        $SiteName = $Site.displayName
        $SiteUrl = $Site.webUrl

        # 4. For each site, Query Drives
        $DrivesUrl = "https://graph.microsoft.com/v1.0/sites/$SiteId/drives"
        
        try {
            $DrivesResponse = Invoke-RestMethod -Uri $DrivesUrl -Headers $Headers -Method Get
            $Drives = $DrivesResponse.value

            foreach ($Drive in $Drives) {
                $DriveId = $Drive.id
                $DriveName = $Drive.name

                # Truncate names for cleaner output
                $DisplaySite = if ($SiteName.Length -gt 33) { $SiteName.Substring(0, 30) + "..." } else { $SiteName }
                $DisplayDrive = if ($DriveName.Length -gt 18) { $DriveName.Substring(0, 15) + "..." } else { $DriveName }

                Write-Host ("{0,-35} | {1,-20} | {2,-20} | {3,-40} | {4}" -f $DisplaySite, $DisplayDrive, $DriveId, $SiteUrl, $SiteId)
            }
        }
        catch {
            Write-Warning "Could not list drives for site: $SiteName"
        }
    }
}
catch {
    Write-Error "API Call Failed: $_"
}
