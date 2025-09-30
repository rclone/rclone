### vfs/status: Get cache status of a VFS. {#vfs-status}

This returns cache status information for the selected VFS, providing
an overview of the cache state including the number of files in each
cache status category.

This is useful for monitoring the overall cache health and usage patterns,
particularly for file manager integrations that need to display cache status
overlays.

    rclone rc vfs/status

The response includes counts for each cache status type:

- "FULL": Files completely cached locally
- "NONE": Files not cached (remote only)
- "PARTIAL": Files partially cached
- "DIRTY": Files modified locally but not uploaded
- "UPLOADING": Files currently being uploaded

Example response:

    {
        "counts": {
            "FULL": 15,
            "NONE": 234,
            "PARTIAL": 3,
            "DIRTY": 2,
            "UPLOADING": 1
        },
        "fs": "/mnt/remote"
    }

This command takes an "fs" parameter. If this parameter is not
supplied and if there is only one VFS in use then that VFS will be
used. If there is more than one VFS in use then the "fs" parameter
must be supplied.

### vfs/file-status: Get cache status for specific files. {#vfs-file-status}

This returns detailed cache status for specific files in the VFS. This is
particularly useful for file manager integrations that need to display
cache status overlays on individual files.

Files are specified using the "file" parameter, which can be repeated
multiple times to query several files at once.

    rclone rc vfs/file-status file=document.pdf file=image.jpg

The response includes cache status, percentage cached (if applicable),
and upload status for each file:

    {
        "files": {
            "document.pdf": {
                "status": "FULL",
                "percentage": 100,
                "uploading": false
            },
            "image.jpg": {
                "status": "PARTIAL", 
                "percentage": 67,
                "uploading": false
            }
        },
        "fs": "/mnt/remote"
    }

Cache status values:
- "FULL": File is completely cached locally
- "NONE": File is not cached (remote only)
- "PARTIAL": File is partially cached
- "DIRTY": File has been modified locally but not uploaded
- "UPLOADING": File is currently being uploaded

The "percentage" field indicates how much of the file is cached locally
(0-100). This is only meaningful for "PARTIAL" status files.

The "uploading" field indicates if the file is currently being uploaded.

This command takes an "fs" parameter. If this parameter is not
supplied and if there is only one VFS in use then that VFS will be
used. If there is more than one VFS in use then the "fs" parameter
must be supplied.

### vfs/dir-status: Get cache status for files in a directory. {#vfs-dir-status}

This returns cache status for all files in a specified directory,
optionally including subdirectories. This is ideal for file manager
integrations that need to display cache status overlays for directory
listings.

The directory is specified using the "dir" parameter. Use "recursive=true"
to include all subdirectories.

    rclone rc vfs/dir-status dir=/documents
    rclone rc vfs/dir-status dir=/documents recursive=true

The response groups files by their cache status and provides detailed
information about each file:

    {
        "dir": "/documents",
        "files": {
            "FULL": [
                {
                    "name": "report.pdf",
                    "percentage": 100,
                    "uploading": false
                }
            ],
            "NONE": [
                {
                    "name": "archive.zip", 
                    "percentage": 0,
                    "uploading": false
                }
            ],
            "PARTIAL": [
                {
                    "name": "video.mp4",
                    "percentage": 45,
                    "uploading": false
                }
            ]
        },
        "fs": "/mnt/remote",
        "recursive": false
    }

Each file entry includes:
- "name": The file name relative to the directory
- "percentage": Cache percentage (0-100)
- "uploading": Whether the file is currently being uploaded

Files are grouped by their cache status for efficient processing by client
applications. The "recursive" field indicates whether subdirectories were
included in the scan.

This command takes an "fs" parameter. If this parameter is not
supplied and if there is only one VFS in use then that VFS will be
used. If there is more than one VFS in use then the "fs" parameter
must be supplied.