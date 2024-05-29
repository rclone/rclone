OneDrive supports System Metadata (not User Metadata, as of this writing) for
both files and directories. Much of the metadata is read-only, and there are some
differences between OneDrive Personal and Business (see table below for
details).

Permissions are also supported, if `--onedrive-metadata-permissions` is set. The
accepted values for `--onedrive-metadata-permissions` are "`read`", "`write`",
"`read,write`", and "`off`" (the default). "`write`" supports adding new permissions,
updating the "role" of existing permissions, and removing permissions. Updating
and removing require the Permission ID to be known, so it is recommended to use
"`read,write`" instead of "`write`" if you wish to update/remove permissions.

Permissions are read/written in JSON format using the same schema as the
[OneDrive API](https://learn.microsoft.com/en-us/onedrive/developer/rest-api/resources/permission?view=odsp-graph-online),
which differs slightly between OneDrive Personal and Business.

Example for OneDrive Personal:
```json
[
	{
		"id": "1234567890ABC!123",
		"grantedTo": {
			"user": {
				"id": "ryan@contoso.com"
			},
			"application": {},
			"device": {}
		},
		"invitation": {
			"email": "ryan@contoso.com"
		},
		"link": {
			"webUrl": "https://1drv.ms/t/s!1234567890ABC"
		},
		"roles": [
			"read"
		],
		"shareId": "s!1234567890ABC"
	}
]
```

Example for OneDrive Business:
```json
[
	{
		"id": "48d31887-5fad-4d73-a9f5-3c356e68a038",
		"grantedToIdentities": [
			{
				"user": {
					"displayName": "ryan@contoso.com"
				},
				"application": {},
				"device": {}
			}
		],
		"link": {
			"type": "view",
			"scope": "users",
			"webUrl": "https://contoso.sharepoint.com/:w:/t/design/a577ghg9hgh737613bmbjf839026561fmzhsr85ng9f3hjck2t5s"
		},
		"roles": [
			"read"
		],
		"shareId": "u!LKj1lkdlals90j1nlkascl"
	},
	{
		"id": "5D33DD65C6932946",
		"grantedTo": {
			"user": {
				"displayName": "John Doe",
				"id": "efee1b77-fb3b-4f65-99d6-274c11914d12"
			},
			"application": {},
			"device": {}
		},
		"roles": [
			"owner"
		],
		"shareId": "FWxc1lasfdbEAGM5fI7B67aB5ZMPDMmQ11U"
	}
]
```

To write permissions, pass in a "permissions" metadata key using this same
format. The [`--metadata-mapper`](https://rclone.org/docs/#metadata-mapper) tool can
be very helpful for this.

When adding permissions, an email address can be provided in the `User.ID` or
`DisplayName` properties of `grantedTo` or `grantedToIdentities`. Alternatively,
an ObjectID can be provided in `User.ID`. At least one valid recipient must be
provided in order to add a permission for a user. Creating a Public Link is also
supported, if `Link.Scope` is set to `"anonymous"`.

Example request to add a "read" permission with `--metadata-mapper`:

```json
{
    "Metadata": {
        "permissions": "[{\"grantedToIdentities\":[{\"user\":{\"id\":\"ryan@contoso.com\"}}],\"roles\":[\"read\"]}]"
    }
}
```

Note that adding a permission can fail if a conflicting permission already
exists for the file/folder.

To update an existing permission, include both the Permission ID and the new
`roles` to be assigned. `roles` is the only property that can be changed.

To remove permissions, pass in a blob containing only the permissions you wish
to keep (which can be empty, to remove all.) Note that the `owner` role will be
ignored, as it cannot be removed.

Note that both reading and writing permissions requires extra API calls, so if
you don't need to read or write permissions it is recommended to omit
`--onedrive-metadata-permissions`.

Metadata and permissions are supported for Folders (directories) as well as
Files. Note that setting the `mtime` or `btime` on a Folder requires one extra
API call on OneDrive Business only.

OneDrive does not currently support User Metadata. When writing metadata, only
writeable system properties will be written -- any read-only or unrecognized keys
passed in will be ignored.

TIP: to see the metadata and permissions for any file or folder, run:

```
rclone lsjson remote:path --stat -M --onedrive-metadata-permissions read
```