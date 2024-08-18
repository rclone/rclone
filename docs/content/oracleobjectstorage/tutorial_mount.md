---
title: "Oracle Object Storage Mount"
description: "Oracle Object Storage mounting tutorial"
---

# {{< icon "fa fa-cloud" >}} Mount Buckets and Expose via NFS Tutorial 
This runbook shows how to [mount](/commands/rclone_mount/) *Oracle Object Storage* buckets as local file system in
OCI compute Instance using rclone tool. 

You will also learn how to export the rclone mounts as NFS mount, so that other NFS client can access them.

Usage Pattern :

NFS Client --> NFS Server --> RClone Mount --> OCI Object Storage

## Step 1 : Install Rclone

In oracle linux 8, Rclone can be installed from
[OL8_Developer](https://yum.oracle.com/repo/OracleLinux/OL8/developer/x86_64/index.html) Yum Repo, Please enable the
repo if not enabled already.

```shell
[opc@base-inst-boot ~]$ sudo yum-config-manager --enable ol8_developer
[opc@base-inst-boot ~]$ sudo yum install -y rclone
[opc@base-inst-boot ~]$ sudo yum install -y fuse
# rclone will prefer fuse3 if available
[opc@base-inst-boot ~]$ sudo yum install -y fuse3
[opc@base-inst-boot ~]$ yum info rclone
Last metadata expiration check: 0:01:58 ago on Fri 07 Apr 2023 05:53:43 PM GMT.
Installed Packages
Name                : rclone
Version             : 1.62.2
Release             : 1.0.1.el8
Architecture        : x86_64
Size                : 67 M
Source              : rclone-1.62.2-1.0.1.el8.src.rpm
Repository          : @System
From repo           : ol8_developer
Summary             : rsync for cloud storage
URL                 : http://rclone.org/
License             : MIT
Description         : Rclone is a command line program to sync files and directories to and from various cloud services.  	
```

To run it as a mount helper you should symlink rclone binary to /sbin/mount.rclone and optionally /usr/bin/rclonefs,
e.g. ln -s /usr/bin/rclone /sbin/mount.rclone. rclone will detect it and translate command-line arguments appropriately.

```shell
ln -s /usr/bin/rclone /sbin/mount.rclone
```

## Step 2: Setup Rclone Configuration file

Let's assume you want to access 3 buckets from the oci compute instance using instance principal provider as means of
authenticating with object storage service.

- namespace-a, bucket-a,
- namespace-b, bucket-b,
- namespace-c, bucket-c

Rclone configuration file needs to have 3 remote sections, one section of each of above 3 buckets. Create a
configuration file in a accessible location that rclone program can read.

```shell

[opc@base-inst-boot ~]$ mkdir -p /etc/rclone
[opc@base-inst-boot ~]$ sudo touch /etc/rclone/rclone.conf
 
 
# add below contents to /etc/rclone/rclone.conf
[opc@base-inst-boot ~]$ cat /etc/rclone/rclone.conf
 
 
[ossa]
type = oracleobjectstorage
provider = instance_principal_auth
namespace = namespace-a
compartment = ocid1.compartment.oc1..aaaaaaaa...compartment-a
region = us-ashburn-1
 
[ossb]
type = oracleobjectstorage
provider = instance_principal_auth
namespace = namespace-b
compartment = ocid1.compartment.oc1..aaaaaaaa...compartment-b
region = us-ashburn-1
 
 
[ossc]
type = oracleobjectstorage
provider = instance_principal_auth
namespace = namespace-c
compartment = ocid1.compartment.oc1..aaaaaaaa...compartment-c
region = us-ashburn-1
 
# List remotes
[opc@base-inst-boot ~]$ rclone --config /etc/rclone/rclone.conf listremotes
ossa:
ossb:
ossc:
 
# Now please ensure you do not see below errors while listing the bucket,
# i.e you should fix the settings to see if namespace, compartment, bucket name are all correct. 
# and you must have a dynamic group policy to allow the instance to use object-family in compartment.
 
[opc@base-inst-boot ~]$ rclone --config /etc/rclone/rclone.conf ls ossa:
2023/04/07 19:09:21 Failed to ls: Error returned by ObjectStorage Service. Http Status Code: 404. Error Code: NamespaceNotFound. Opc request id: iad-1:kVVAb0knsVXDvu9aHUGHRs3gSNBOFO2_334B6co82LrPMWo2lM5PuBKNxJOTmZsS. Message: You do not have authorization to perform this request, or the requested resource could not be found.
Operation Name: ListBuckets
Timestamp: 2023-04-07 19:09:21 +0000 GMT
Client Version: Oracle-GoSDK/65.32.0
Request Endpoint: GET https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace-a/b?compartmentId=ocid1.compartment.oc1..aaaaaaaa...compartment-a
Troubleshooting Tips: See https://docs.oracle.com/iaas/Content/API/References/apierrors.htm#apierrors_404__404_namespacenotfound for more information about resolving this error.
Also see https://docs.oracle.com/iaas/api/#/en/objectstorage/20160918/Bucket/ListBuckets for details on this operation's requirements.
To get more info on the failing request, you can set OCI_GO_SDK_DEBUG env var to info or higher level to log the request/response details.
If you are unable to resolve this ObjectStorage issue, please contact Oracle support and provide them this full error message.
[opc@base-inst-boot ~]$

```

## Step 3: Setup Dynamic Group and Add IAM Policy.
Just like a human user has an identity identified by its USER-PRINCIPAL, every OCI compute instance is also a robotic
user identified by its INSTANCE-PRINCIPAL. The instance principal key is automatically fetched by rclone/with-oci-sdk
from instance-metadata to make calls to object storage.

Similar to [user-group](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managinggroups.htm),
[instance groups](https://docs.oracle.com/en-us/iaas/Content/Identity/Tasks/managingdynamicgroups.htm)
is known as dynamic-group in IAM.

Create a dynamic group say rclone-dynamic-group that the oci compute instance becomes a member of the below group
says all instances belonging to compartment a...c is member of this dynamic-group.

```shell
any {instance.compartment.id = '<compartment_ocid_a>', 
     instance.compartment.id = '<compartment_ocid_b>', 
     instance.compartment.id = '<compartment_ocid_c>'
    }
```

Now that you have a dynamic group, you need to add a policy allowing what permissions this dynamic-group has.
In our case, we want this dynamic-group to access object-storage. So create a policy now.

```shell
allow dynamic-group rclone-dynamic-group to manage object-family in compartment compartment-a
allow dynamic-group rclone-dynamic-group to manage object-family in compartment compartment-b
allow dynamic-group rclone-dynamic-group to manage object-family in compartment compartment-c
```

After you add the policy, now ensure the rclone can list files in your bucket, if not please troubleshoot any mistakes
you did so far. Please note, identity can take upto a minute to ensure policy gets reflected.

## Step 4: Setup Mount Folders
Let's assume you have to mount 3 buckets, bucket-a, bucket-b, bucket-c at path /opt/mnt/bucket-a, /opt/mnt/bucket-b,
/opt/mnt/bucket-c respectively.

Create the mount folder and set its ownership to desired user, group.
```shell
[opc@base-inst-boot ~]$ sudo mkdir /opt/mnt
[opc@base-inst-boot ~]$ sudo chown -R opc:adm /opt/mnt
```

Set chmod permissions to user, group, others as desired for each mount path
```shell
[opc@base-inst-boot ~]$ sudo chmod 764 /opt/mnt
[opc@base-inst-boot ~]$ ls -al /opt/mnt/
total 0
drwxrw-r--. 2 opc adm 6 Apr 7 18:01 .
drwxr-xr-x. 10 root root 179 Apr 7 18:01 ..

[opc@base-inst-boot ~]$ mkdir -p /opt/mnt/bucket-a
[opc@base-inst-boot ~]$ mkdir -p /opt/mnt/bucket-b
[opc@base-inst-boot ~]$ mkdir -p /opt/mnt/bucket-c

[opc@base-inst-boot ~]$ ls -al /opt/mnt
total 0
drwxrw-r--. 5 opc adm 54 Apr 7 18:17 .
drwxr-xr-x. 10 root root 179 Apr 7 18:01 ..
drwxrwxr-x. 2 opc opc 6 Apr 7 18:17 bucket-a
drwxrwxr-x. 2 opc opc 6 Apr 7 18:17 bucket-b
drwxrwxr-x. 2 opc opc 6 Apr 7 18:17 bucket-c
```

## Step 5: Identify Rclone mount CLI configuration settings to use.
Please read through this [rclone mount](https://rclone.org/commands/rclone_mount/) page completely to really
understand the mount and its flags, what is rclone
[virtual file system](https://rclone.org/commands/rclone_mount/#vfs-virtual-file-system) mode settings and
how to effectively use them for desired Read/Write consistencies.

Local File systems expect things to be 100% reliable, whereas cloud storage systems are a long way from 100% reliable.
Object storage can throw several errors like 429, 503, 404 etc. The rclone sync/copy commands cope with this with
lots of retries. However rclone mount can't use retries in the same way without making local copies of the uploads.
Please Look at the VFS File Caching for solutions to make mount more reliable.

First lets understand the rclone mount flags and some global flags for troubleshooting.

```shell

rclone mount \
    ossa:bucket-a \                     # Remote:bucket-name
    /opt/mnt/bucket-a \                 # Local mount folder
    --config /etc/rclone/rclone.conf \  # Path to rclone config file
    --allow-non-empty \                 # Allow mounting over a non-empty directory
    --dir-perms 0770 \                  # Directory permissions (default 0777)
    --file-perms 0660 \                 # File permissions (default 0666)
    --allow-other \                     # Allow access to other users
    --umask 0117  \                     # sets (660) rw-rw---- as permissions for the mount using the umask
    --transfers 8 \                     # default 4, can be set to adjust the number of parallel uploads of modified files to remote from the cache
    --tpslimit 50  \                    # Limit HTTP transactions per second to this. A transaction is roughly defined as an API call;
                                        # its exact meaning will depend on the backend. For HTTP based backends it is an HTTP PUT/GET/POST/etc and its response
    --cache-dir /tmp/rclone/cache       # Directory rclone will use for caching.
    --dir-cache-time 5m \               # Time to cache directory entries for (default 5m0s)
    --vfs-cache-mode writes \           # Cache mode off|minimal|writes|full (default off), writes gives the maximum compatibility like a local disk
    --vfs-cache-max-age 20m \           # Max age of objects in the cache (default 1h0m0s)
    --vfs-cache-max-size 10G \          # Max total size of objects in the cache (default off)
    --vfs-cache-poll-interval 1m \      # Interval to poll the cache for stale objects (default 1m0s)
    --vfs-write-back 5s   \             # Time to writeback files after last use when using cache (default 5s). 
                                        # Note that files are written back to the remote only when they are closed and
                                        # if they haven't been accessed for --vfs-write-back seconds. If rclone is quit or
                                        # dies with files that haven't been uploaded, these will be uploaded next time rclone is run with the same flags.
    --vfs-fast-fingerprint              # Use fast (less accurate) fingerprints for change detection.    
    --log-level ERROR \                            # log level, can be DEBUG, INFO, ERROR
    --log-file /var/log/rclone/oosa-bucket-a.log   # rclone application log
    
```

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from the remote, write only and read/write files are
buffered to disk first. This mode should support all normal file system operations. If an upload fails it will be
retried at exponentially increasing intervals up to 1 minute.

VFS cache mode of writes is recommended, so that application can have maximum compatibility of using remote storage
as a local disk, when write is finished, file is closed, it is uploaded to backend remote after vfs-write-back duration
has elapsed. If rclone is quit or dies with files that haven't been uploaded, these will be uploaded next time rclone
is run with the same flags.

### --tpslimit float

Limit transactions per second to this number. Default is 0 which is used to mean unlimited transactions per second.

A transaction is roughly defined as an API call; its exact meaning will depend on the backend. For HTTP based backends
it is an HTTP PUT/GET/POST/etc and its response. For FTP/SFTP it is a round trip transaction over TCP.

For example, to limit rclone to 10 transactions per second use --tpslimit 10, or to 1 transaction every 2 seconds
use --tpslimit 0.5.

Use this when the number of transactions per second from rclone is causing a problem with the cloud storage
provider (e.g. getting you banned or rate limited or throttled).

This can be very useful for rclone mount to control the behaviour of applications using it. Let's guess and say Object
storage allows roughly 100 tps per tenant, so to be on safe side, it will be wise to set this at 50. (tune it to actuals per
region)

### --vfs-fast-fingerprint

If you use the --vfs-fast-fingerprint flag then rclone will not include the slow operations in the fingerprint. This
makes the fingerprinting less accurate but much faster and will improve the opening time of cached files. If you are
running a vfs cache over local, s3, object storage or swift backends then using this flag is recommended.

Various parts of the VFS use fingerprinting to see if a local file copy has changed relative to a remote file.
Fingerprints are made from:
- size
- modification time
- hash
  where available on an object.


## Step 6: Mounting Options, Use Any one option

### Step 6a: Run as a Service Daemon: Configure FSTAB entry for Rclone mount
Add this entry in /etc/fstab :
```shell
ossa:bucket-a /opt/mnt/bucket-a rclone rw,umask=0117,nofail,_netdev,args2env,config=/etc/rclone/rclone.conf,uid=1000,gid=4,
file_perms=0760,dir_perms=0760,allow_other,vfs_cache_mode=writes,cache_dir=/tmp/rclone/cache 0 0
```
IMPORTANT: Please note in fstab entry arguments are specified as underscore instead of dash,
example: vfs_cache_mode=writes instead of vfs-cache-mode=writes
Rclone in the mount helper mode will split -o argument(s) by comma, replace _ by - and prepend -- to
get the command-line flags. Options containing commas or spaces can be wrapped in single or double quotes.
Any inner quotes inside outer quotes of the same type should be doubled.


then run sudo mount -av
```shell

[opc@base-inst-boot ~]$ sudo mount -av
/                    : ignored
/boot                : already mounted
/boot/efi            : already mounted
/var/oled            : already mounted
/dev/shm             : already mounted
none                 : ignored
/opt/mnt/bucket-a    : already mounted   # This is the bucket mounted information, running mount -av again and again is idempotent.

```

## Step 6b: Run as a Service Daemon: Configure systemd entry for Rclone mount

If you are familiar with configuring systemd unit files, you can also configure the each rclone mount into a
systemd units file.
various examples in git search: https://github.com/search?l=Shell&q=rclone+unit&type=Code
```shell
tee "/etc/systemd/system/rclonebucketa.service" > /dev/null <<EOF
[Unit]
Description=RCloneMounting
After=multi-user.target
[Service]
Type=simple
User=0
Group=0
ExecStart=/bin/bash /etc/rclone/scripts/bucket-a.sh
ExecStop=/bin/fusermount -uz /opt/mnt/bucket-a
TimeoutStopSec=20
KillMode=process
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
EOF
```

## Step 7: Optional: Mount Nanny, for resiliency, recover from process crash.
Sometimes, rclone process crashes and the mount points are left in dangling state where its mounted but the rclone
mount process is gone. To clean up the mount point you can force unmount by running this command.
```shell
sudo fusermount -uz /opt/mnt/bucket-a
```
One can also run a rclone_mount_nanny script, which detects and cleans up mount errors by unmounting and
then auto-mounting.

Content of /etc/rclone/scripts/rclone_nanny_script.sh
```shell

#!/usr/bin/env bash
erroneous_list=$(df 2>&1 | grep -i 'Transport endpoint is not connected' | awk '{print ""$2"" }' | tr -d \:)
rclone_list=$(findmnt -t fuse.rclone -n 2>&1 | awk '{print ""$1"" }' | tr -d \:)
IFS=$'\n'; set -f
intersection=$(comm -12 <(printf '%s\n' "$erroneous_list" | sort) <(printf '%s\n' "$rclone_list" | sort))
for directory in $intersection
do
    echo "$directory is being fixed."
    sudo fusermount -uz "$directory"
done
sudo mount -av

```
Script to idempotently add a Cron job to babysit the mount paths every 5 minutes
```shell
echo "Creating rclone nanny cron job."
croncmd="/etc/rclone/scripts/rclone_nanny_script.sh"
cronjob="*/5 * * * * $croncmd"
# idempotency - adds rclone_nanny cronjob only if absent.
( crontab -l | grep -v -F "$croncmd" || : ; echo "$cronjob" ) | crontab -
echo "Finished creating rclone nanny cron job."
```

Ensure the crontab is added, so that above nanny script runs every 5 minutes.
```shell
[opc@base-inst-boot ~]$ sudo crontab -l
*/5 * * * * /etc/rclone/scripts/rclone_nanny_script.sh
[opc@base-inst-boot ~]$  
```

## Step 8: Optional: Setup NFS server to access the mount points of rclone

Let's say you want to make the rclone mount path /opt/mnt/bucket-a available as a NFS server export so that other
clients can access it by using a NFS client.

### Step 8a : Setup NFS server

Install NFS Utils
```shell
sudo yum install -y nfs-utils
```

Export the desired directory via NFS Server in the same machine where rclone has mounted to, ensure NFS service has
desired permissions to read the directory. If it runs as root, then it will have permissions for sure, but if it runs
as separate user then ensure that user has necessary desired privileges.
```shell
# this gives opc user and adm (administrators group) ownership to the path, so any user belonging to adm group will be able to access the files.
[opc@tools ~]$ sudo chown -R opc:adm /opt/mnt/bucket-a/
[opc@tools ~]$ sudo chmod 764 /opt/mnt/bucket-a/
 
# Not export the mount path of rclone for exposing via nfs server
# There are various nfs export options that you should keep per desired usage.
# Syntax is
# <path> <allowed-ipaddr>(<option>)
[opc@tools ~]$ cat /etc/exports
/opt/mnt/bucket-a *(fsid=1,rw)
 
 
# Restart NFS server
[opc@tools ~]$ sudo systemctl restart nfs-server
 
 
# Show Export paths
[opc@tools ~]$ showmount -e
Export list for tools:
/opt/mnt/bucket-a *
 
# Know the port NFS server is running as, in this case it's listening on port 2049
[opc@tools ~]$ sudo rpcinfo -p | grep nfs
100003 3 tcp 2049 nfs
100003 4 tcp 2049 nfs
100227 3 tcp 2049 nfs_acl
 
# Allow NFS service via firewall
[opc@tools ~]$ sudo firewall-cmd --add-service=nfs --permanent
Warning: ALREADY_ENABLED: nfs
success
[opc@tools ~]$ sudo firewall-cmd --reload
success
[opc@tools ~]$
 
# Check status of NFS service
[opc@tools ~]$ sudo systemctl status nfs-server.service
● nfs-server.service - NFS server and services
   Loaded: loaded (/usr/lib/systemd/system/nfs-server.service; enabled; vendor preset: disabled)
   Active: active (exited) since Wed 2023-04-19 17:59:58 GMT; 13min ago
  Process: 2833741 ExecStopPost=/usr/sbin/exportfs -f (code=exited, status=0/SUCCESS)
  Process: 2833740 ExecStopPost=/usr/sbin/exportfs -au (code=exited, status=0/SUCCESS)
  Process: 2833737 ExecStop=/usr/sbin/rpc.nfsd 0 (code=exited, status=0/SUCCESS)
  Process: 2833766 ExecStart=/bin/sh -c if systemctl -q is-active gssproxy; then systemctl reload gssproxy ; fi (code=exit>
  Process: 2833756 ExecStart=/usr/sbin/rpc.nfsd (code=exited, status=0/SUCCESS)
  Process: 2833754 ExecStartPre=/usr/sbin/exportfs -r (code=exited, status=0/SUCCESS)
 Main PID: 2833766 (code=exited, status=0/SUCCESS)
    Tasks: 0 (limit: 48514)
   Memory: 0B
   CGroup: /system.slice/nfs-server.service
 
Apr 19 17:59:58 tools systemd[1]: Starting NFS server and services...
Apr 19 17:59:58 tools systemd[1]: Started NFS server and services.  
```

### Step 8b : Setup NFS client

Now to connect to the NFS server from a different client machine, ensure the client machine can reach to nfs server machine over tcp port 2049, ensure your subnet network acls allow from desired source IP ranges to destination:2049 port.

In the client machine Mount the external NFS

```shell
# Install nfs-utils
[opc@base-inst-boot ~]$ sudo yum install -y nfs-utils
 
# In /etc/fstab, add the below entry
[opc@base-inst-boot ~]$ cat /etc/fstab | grep nfs
<ProvideYourIPAddress>:/opt/mnt/bucket-a /opt/mnt/buckert-a nfs rw 0 0
 
# remount so that newly added path gets mounted.
[opc@base-inst-boot ~]$ sudo mount -av
/ : ignored
/boot : already mounted
/boot/efi : already mounted
/var/oled : already mounted
/dev/shm : already mounted
/home/opc/share_drive/bucketa: already mounted
/opt/mnt/bucket-a: successfully mounted # this is the NFS mount
```

### Step 8c : Test Connection

```shell
# List files to test connection
[opc@base-inst-boot ~]$ ls -al /opt/mnt/bucket-a
total 1
drw-rw----. 1 opc adm 0 Apr 18 17:28 .
drwxrw-r--. 7 opc adm 85 Apr 18 17:36 ..
drw-rw----. 1 opc adm 0 Apr 18 17:29 FILES
-rw-rw----. 1 opc adm 15 Apr 18 18:13 nfs.txt
```


