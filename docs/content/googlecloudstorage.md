---
title: "Google Cloud Storage"
description: "Rclone docs for Google Cloud Storage"
date: "2015-09-12"
---

<i class="fa fa-google"></i> Google Cloud Storage
-------------------------------------------------

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

The initial setup for google cloud storage involves getting a token from Google Cloud Storage
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Cloud Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Dropbox
   \ "dropbox"
 5 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 6 / Google Drive
   \ "drive"
 7 / Hubic
   \ "hubic"
 8 / Local Disk
   \ "local"
 9 / Microsoft OneDrive
   \ "onedrive"
10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
11 / Yandex Disk
   \ "yandex"
Storage> 5
Google Application Client Id - leave blank normally.
client_id> 
Google Application Client Secret - leave blank normally.
client_secret> 
Project number optional - needed only for list/create/delete buckets - see your developer console.
project_number> 12345678
Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.
service_account_file> 
Access Control List for new objects.
Choose a number from below, or type in your own value
 * Object owner gets OWNER access, and all Authenticated Users get READER access.
 1) authenticatedRead
 * Object owner gets OWNER access, and project team owners get OWNER access.
 2) bucketOwnerFullControl
 * Object owner gets OWNER access, and project team owners get READER access.
 3) bucketOwnerRead
 * Object owner gets OWNER access [default if left blank].
 4) private
 * Object owner gets OWNER access, and project team members get access according to their roles.
 5) projectPrivate
 * Object owner gets OWNER access, and all Users get READER access.
 6) publicRead
object_acl> 4
Access Control List for new buckets.
Choose a number from below, or type in your own value
 * Project team owners get OWNER access, and all Authenticated Users get READER access.
 1) authenticatedRead
 * Project team owners get OWNER access [default if left blank].
 2) private
 * Project team members get access according to their roles.
 3) projectPrivate
 * Project team owners get OWNER access, and all Users get READER access.
 4) publicRead
 * Project team owners get OWNER access, and all Users get WRITER access.
 5) publicReadWrite
bucket_acl> 2
Remote config
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine or Y didn't work
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = google cloud storage
client_id = 
client_secret = 
token = {"AccessToken":"xxxx.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"x/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx_xxxxxxxxx","Expiry":"2014-07-17T20:49:14.929208288+01:00","Extra":null}
project_number = 12345678
object_acl = private
bucket_acl = private
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
it may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

This remote is called `remote` and can now be used like this

See all the buckets in your project

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

### Service Account support ###

You can set up rclone with Google Cloud Storage in an unattended mode,
i.e. not tied to a specific end-user Google account. This is useful
when you want to synchronise files onto machines that don't have
actively logged-in users, for example build machines.

To get credentials for Google Cloud Platform
[IAM Service Accounts](https://cloud.google.com/iam/docs/service-accounts),
please head to the
[Service Account](https://console.cloud.google.com/permissions/serviceaccounts)
section of the Google Developer Console. Service Accounts behave just
like normal `User` permissions in
[Google Cloud Storage ACLs](https://cloud.google.com/storage/docs/access-control),
so you can limit their access (e.g. make them read only). After
creating an account, a JSON file containing the Service Account's
credentials will be downloaded onto your machines. These credentials
are what rclone will use for authentication.

To use a Service Account instead of OAuth2 token flow, enter the path
to your Service Account credentials at the `service_account_file`
prompt and rclone won't use the browser based authentication
flow.

### Modified time ###

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the "mtime" key in
RFC3339 format accurate to 1ns.

## Service Account support ##

You can set up `rcloud` with Google Cloud Storage in an unattended mode, i.e. not tied to a specific end-user Google 
account. This is useful when you want to synchronise files onto machines that don't have actively logged-in users, for 
example build machines.

To get credentials for Google Cloud Platform [IAM Service Accounts](https://cloud.google.com/iam/docs/service-accounts),
please head to the [Service Account](https://console.cloud.google.com/permissions/serviceaccounts) section of the 
Google Developer Console. Service Accounts behave just like normal `User` permissions in 
[Google Cloud Storage ACLs](https://cloud.google.com/storage/docs/access-control), so you can limit their access 
(e.g. make them read only). After creating an account, a JSON file containing the Service Account's credentials will 
be downloaded onto your machines. These credentials are what `rclone` will use for authentication.

To use a Service Account instead of OAuth2 token flow, replace the `token` section of your `.rclone.conf` with a
 `service_account_file` pointing to the JSON credentials. 

For example, here's an example `.rclone.conf` that sets up read only access using a service account:

```
[readonly-sync]
type = google cloud storage
project_number = 123456789
service_account_file = $HOME/.rclone-service_account.json
object_acl = authenticatedRead
bucket_acl = authenticatedRead
```


