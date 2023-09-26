# Automatic Syncing from Google Drive to Server with rclone

This guide provides a step-by-step process to set up `rclone` for automatically syncing files from Google Drive to a server directory and for setting up a cron job for periodic syncing.

## Step 1: Install rclone

First, you need to install `rclone`. On most Linux distributions, you can use:

```bash
curl https://rclone.org/install.sh | sudo bash
```

## Step 2: Google Cloud Setup for rclone

### 2.1. Create a New Google Cloud Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/).
2. Click the project dropdown, then click on "+ NEW PROJECT".
3. Provide a name for your project and click "CREATE".

### 2.2. Enable Google Drive API

1. In the Dashboard of your new project, click on "Navigation Menu" (three horizontal lines at the top-left).
2. Navigate to **APIs & Services** > **Library**.
3. Search for "Google Drive API" and select it.
4. Click on the "ENABLE" button.

### 2.3. Create Credentials

1. On the Google Drive API page, click "Create Credentials".
2. For "Which API are you using?", select "Google Drive API".
3. For "Where will you be calling the API from?", select "Other non-UI (e.g. daemon, cron job, etc.)".
4. For "What data will you be accessing?", select "Application data".
5. Click on the "What credentials do I need?" button.
6. It will prompt you to create a Service Account. Fill in the details.
7. After creating, it'll ask if you want to grant this service account access to the project. You can select a role like "Editor" if you want it to have edit permissions on Google Drive. Or, choose "Viewer" for read-only.
8. Continue and create the key as JSON. Save this file securely; you'll use it with rclone.

### 2.4. Share Drive Folder with Service Account

1. Go to your Google Drive.
2. Find the folder you want to sync with rclone.
3. Right-click on the folder and go to "Share".
4. Add your Service Account email (which looks something like `your-service-account@your-project-id.iam.gserviceaccount.com`).
5. Grant the desired permissions (View/Edit).

### 2.5. Use the Service Account Key with rclone

When you're configuring rclone, you'll have the option to provide a Service Account key. Use the path to the JSON file you downloaded earlier.

> **Note:** When using rclone with this configuration, it'll access Google Drive with the permissions and credentials of the Service Account. Always keep the JSON key file safe and do not share it publicly.

## Step 3: Configure rclone for Google Drive

Run the configuration command:

```bash
rclone config
```

During the configuration:

1. Choose **"n"** for a new remote.
2. Name the remote (for example, "gdrive").
3. Select the number corresponding to **"Google Drive"**.
4. Leave the **"client_id"** and **"client_secret"** blank (unless you have specific ones to use).
5. Choose **"1"** for full access.
6. Say **"n"** to advanced config.
7. Choose **"n"** for auto config (especially if you're on a remote server without a GUI).
8. Open the provided link in a web browser, authenticate with your Google account, and paste the verification code back into the terminal.
9. Confirm the configuration by choosing **"y"**.

You should now have rclone configured for your Google Drive.

## Step 4: Test the rclone Configuration

To ensure your Google Drive is accessible via `rclone`, use the `ls` command:

```bash
rclone ls gdrive:
```

This should list files and directories in the root of your Google Drive.

## Step 5: Copy Files

To copy files from Google Drive to a server directory, use:

```bash
rclone copy gdrive:/ /path/to/server/directory/ --include "*.{jpg,jpeg,png,gif,webp,tiff,bmp,svg,mp4,mkv,avi,mp3,flac}"
```

This command ensures only media-related files are copied.

## Step 6: Set Up Automation with Cron

Open the crontab for editing:

```bash
crontab -e
```

Add the following line to set up syncing every 2 hours (adjust timing as needed):

```bash
0 */2 * * * /path/to/rclone copy gdrive:/ /path/to/server/directory/ --max-depth 1 --include "*.{jpg,jpeg,png,gif,webp,tiff,bmp,svg,mp4,mkv,avi,mp3,flac}" >> /path/to/logfile.log 2>&1
```

Save and exit the crontab editor.

## Debugging

If you encounter issues with the setup, here are some steps to help diagnose:

1. **Incorrect Folder ID**: Ensure you've configured rclone with the correct folder ID from Google Drive.
2. **Check rclone Version**: Using outdated versions might cause issues. Ensure you're using the latest version of rclone.
3. **Log Files**: Examine the log file specified in the cron job for errors or messages.
4. **File Extensions**: Ensure you've included all desired file extensions in the `--include` parameter.

## Conclusion

This guide assists in setting up automatic syncing from Google Drive to a server. With this, regularly syncing files becomes effortless, ensuring you always have the latest files from Google Drive in your server directory.
