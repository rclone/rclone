set RCLONE_CONFIG_LOCALWINDOWS_TYPE=local
rclone.exe purge    LocalWindows:info
rclone.exe info -vv LocalWindows:info --write-json=info-LocalWindows.json > info-LocalWindows.log  2>&1
rclone.exe ls   -vv LocalWindows:info > info-LocalWindows.list 2>&1
