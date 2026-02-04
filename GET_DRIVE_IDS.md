# Getting SharePoint Drive IDs for Rclone

Rclone's `onedrive` backend (which supports SharePoint) normally maps one remote to one Document Library. To copy files from multiple SharePoint sites without creating a separate config for each one, you can use a single "base" remote and override the target drive using the `--onedrive-drive-id` flag.

This guide provides scripts to automatically list the **Drive ID** (Document Library ID) for all your accessible SharePoint sites.

## Prerequisites

1.  **Rclone Configured**: You must have at least one valid `onedrive` remote configured in rclone that can access your SharePoint tenant.
    *   Example remote name: `mysharepoint`
2.  **Fresh Token**: Before running these scripts, ensure your token is active by running:
    ```bash
    rclone about mysharepoint:
    ```

---

## Option 1: PowerShell (Windows/Cross-platform)

Save the following code as `Get-OneDriveIDs.ps1`.

### Script

```powershell
# --- CONFIGURATION ---
$RemoteName = "mysharepoint"  # Name of your rclone remote
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
Write-Host ("{0,-40} | {1,-20} | {2}" -f "SITE NAME", "DRIVE NAME", "DRIVE ID (Use in rclone)")
Write-Host ("-" * 100)

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

        # 4. For each site, Query Drives
        $DrivesUrl = "https://graph.microsoft.com/v1.0/sites/$SiteId/drives"
        
        try {
            $DrivesResponse = Invoke-RestMethod -Uri $DrivesUrl -Headers $Headers -Method Get
            $Drives = $DrivesResponse.value

            foreach ($Drive in $Drives) {
                $DriveId = $Drive.id
                $DriveName = $Drive.name

                # Truncate names for cleaner output
                $DisplaySite = if ($SiteName.Length -gt 38) { $SiteName.Substring(0, 35) + "..." } else { $SiteName }
                $DisplayDrive = if ($DriveName.Length -gt 18) { $DriveName.Substring(0, 15) + "..." } else { $DriveName }

                Write-Host ("{0,-40} | {1,-20} | {2}" -f $DisplaySite, $DisplayDrive, $DriveId)
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
```

### Usage

1.  Edit the `$RemoteName` variable in the script to match your rclone remote.
2.  Run the script:
    ```powershell
    .\Get-OneDriveIDs.ps1
    ```

---

## Option 2: Python (macOS/Linux)

Save the following code as `list_drives.py`.

### Script

```python
import subprocess
import json
import urllib.request
import urllib.parse
import sys

# --- CONFIGURATION ---
REMOTE_NAME = "mysharepoint"  # Name of your rclone remote
# ---------------------

def get_rclone_token(remote_name):
    """Gets the access token from rclone config dump."""
    try:
        # Run rclone config dump to get the JSON config
        result = subprocess.run(['rclone', 'config', 'dump'], capture_output=True, text=True)
        config = json.loads(result.stdout)
        
        if remote_name not in config:
            print(f"Error: Remote '{remote_name}' not found in rclone config.")
            sys.exit(1)
            
        token_str = config[remote_name].get('token')
        if not token_str:
            print(f"Error: No token found for '{remote_name}'. Try running 'rclone about {remote_name}:' first.")
            sys.exit(1)
            
        token_data = json.loads(token_str)
        return token_data['access_token']
    except Exception as e:
        print(f"Error getting token: {e}")
        sys.exit(1)

def api_get(url, token):
    """Helper to make Graph API calls."""
    req = urllib.request.Request(url)
    req.add_header('Authorization', f'Bearer {token}')
    try:
        with urllib.request.urlopen(req) as response:
            return json.load(response)
    except urllib.error.HTTPError as e:
        print(f"API Error ({url}): {e}")
        return None

def main():
    token = get_rclone_token(REMOTE_NAME)
    print(f"Successfully retrieved token for '{REMOTE_NAME}'\n")
    print(f"{'SITE NAME':<40} | {'DRIVE NAME':<20} | {'DRIVE ID (Use this in rclone)'}")
    print("-" * 110)

    # 1. Search for all sites
    sites_url = "https://graph.microsoft.com/v1.0/sites?search=*"
    sites_data = api_get(sites_url, token)

    if not sites_data:
        return

    for site in sites_data.get('value', []):
        site_id = site['id']
        site_name = site.get('displayName', 'Unknown')
        
        # 2. For each site, list its drives
        drives_url = f"https://graph.microsoft.com/v1.0/sites/{site_id}/drives"
        drives_data = api_get(drives_url, token)
        
        if drives_data:
            for drive in drives_data.get('value', []):
                drive_id = drive['id']
                drive_name = drive.get('name', 'Documents')
                
                # Print the result row
                print(f"{site_name[:39]:<40} | {drive_name[:19]:<20} | {drive_id}")

if __name__ == "__main__":
    main()
```

### Usage

1.  Edit `REMOTE_NAME` in the script to match your rclone remote.
2.  Run the script:
    ```bash
    python3 list_drives.py
    ```

---

## How to Use the Output

Once you have the Drive ID (e.g., `b!7F3...a1-9c`), you can use it to copy files from that specific site using your base remote:

```bash
rclone copy "mysharepoint:" /local/backup/marketing \
    --onedrive-drive-id "b!7F3...a1-9c"
```
