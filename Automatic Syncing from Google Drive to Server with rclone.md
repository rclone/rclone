## Automatic Syncing from Google Drive to Server with rclone

This guide provides a step-by-step process to set up `rclone` to automatically sync files from Google Drive to a server directory and how to set up a cron job for periodic syncing.

### Step 1: Install rclone

First, you need to install `rclone`. On most Linux distributions, you can use:

```bash
curl https://rclone.org/install.sh | sudo bash
```

### Step 2: Configure rclone for Google Drive

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

Now, you should have rclone configured for your Google Drive.

### Step 3: Test the rclone Configuration

To make sure your Google Drive is accessible via `rclone`, use the `ls` command:

```bash
rclone ls gdrive:
```

This should list files and directories in the root of your Google Drive.

### Step 4: Copy Files

To copy files from Google Drive to a server directory, use:

```bash
rclone copy gdrive:/ /path/to/server/directory/ --include "*.{jpg,jpeg,png,gif,webp,tiff,bmp,svg,mp4,mkv,avi,mp3,flac}"
```

This command ensures only media-related files get copied.

### Step 5: Set Up Automation with Cron

Open the crontab for editing:

```bash
crontab -e
```

Add the following line to set up syncing every 2 hours (adjust timing as needed):

```bash
0 */2 * * * /path/to/rclone copy gdrive:/ /path/to/server/directory/ --max-depth 1 --include "*.{jpg,jpeg,png,gif,webp,tiff,bmp,svg,mp4,mkv,avi,mp3,flac}" >> /path/to/logfile.log 2>&1
```

Save and exit the crontab editor.

### Debugging

If you encounter issues with the setup, here are some steps to help diagnose:

1. **Incorrect Folder ID**: Ensure you've configured rclone with the correct folder ID from Google Drive.
2. **Check rclone Version**: Using outdated versions might lead to issues. Ensure you're using the latest version of rclone.
3. **Log Files**: Examine the log file specified in the cron job for errors or messages.
4. **File Extensions**: Ensure you've included all desired file extensions in the `--include` parameter.

### Conclusion

This guide assists in setting up automatic syncing from Google Drive to a server. Regularly syncing files becomes effortless, ensuring you always have the latest files from Google Drive in your server directory.

---

