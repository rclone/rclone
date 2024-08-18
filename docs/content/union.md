---
title: "Union"
description: "Remote Unification"
versionIntroduced: "v1.44"
---

# {{< icon "fa fa-link" >}} Union

The `union` backend joins several remotes together to make a single unified view of them.

During the initial setup with `rclone config` you will specify the upstream
remotes as a space separated list. The upstream remotes can either be a local
paths or other remotes.

The attributes `:ro`, `:nc` and `:writeback` can be attached to the end of the remote
to tag the remote as **read only**, **no create** or **writeback**, e.g.
`remote:directory/subdirectory:ro` or `remote:directory/subdirectory:nc`.

- `:ro` means files will only be read from here and never written
- `:nc` means new files or directories won't be created here
- `:writeback` means files found in different remotes will be written back here. See the [writeback section](#writeback) for more info.

Subfolders can be used in upstream remotes. Assume a union remote named `backup`
with the remotes `mydrive:private/backup`. Invoking `rclone mkdir backup:desktop`
is exactly the same as invoking `rclone mkdir mydrive:private/backup/desktop`.

There is no special handling of paths containing `..` segments. Invoking `rclone
mkdir backup:../desktop` is exactly the same as invoking `rclone mkdir
mydrive:private/backup/../desktop`.

## Configuration

Here is an example of how to make a union called `remote` for local folders.
First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Union merges the contents of several remotes
   \ "union"
[snip]
Storage> union
List of space separated upstreams.
Can be 'upstreama:test/dir upstreamb:', '\"upstreama:test/space:ro dir\" upstreamb:', etc.
Enter a string value. Press Enter for the default ("").
upstreams> remote1:dir1 remote2:dir2 remote3:dir3
Policy to choose upstream on ACTION class.
Enter a string value. Press Enter for the default ("epall").
action_policy>
Policy to choose upstream on CREATE class.
Enter a string value. Press Enter for the default ("epmfs").
create_policy>
Policy to choose upstream on SEARCH class.
Enter a string value. Press Enter for the default ("ff").
search_policy>
Cache time of usage and free space (in seconds). This option is only useful when a path preserving policy is used.
Enter a signed integer. Press Enter for the default ("120").
cache_time>
Remote config
Configuration complete.
Options:
- type: union
- upstreams: remote1:dir1 remote2:dir2 remote3:dir3
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               union

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

Once configured you can then use `rclone` like this,

List directories in top level in `remote1:dir1`, `remote2:dir2` and `remote3:dir3`

    rclone lsd remote:

List all the files in `remote1:dir1`, `remote2:dir2` and `remote3:dir3`

    rclone ls remote:

Copy another local directory to the union directory called source, which will be placed into `remote3:dir3`

    rclone copy C:\source remote:source

### Behavior / Policies

The behavior of union backend is inspired by [trapexit/mergerfs](https://github.com/trapexit/mergerfs). All functions are grouped into 3 categories: **action**, **create** and **search**. These functions and categories can be assigned a policy which dictates what file or directory is chosen when performing that behavior. Any policy can be assigned to a function or category though some may not be very useful in practice. For instance: **rand** (random) may be useful for file creation (create) but could lead to very odd behavior if used for `delete` if there were more than one copy of the file.

### Function / Category classifications

| Category | Description              | Functions                                                                           |
|----------|--------------------------|-------------------------------------------------------------------------------------|
| action   | Writing Existing file    | move, rmdir, rmdirs, delete, purge and copy, sync (as destination when file exist)  |
| create   | Create non-existing file | copy, sync (as destination when file not exist)                                     |
| search   | Reading and listing file | ls, lsd, lsl, cat, md5sum, sha1sum and copy, sync (as source)                       |
| N/A      |                          | size, about                                                                         |

### Path Preservation

Policies, as described below, are of two basic types. `path preserving` and `non-path preserving`.

All policies which start with `ep` (**epff**, **eplfs**, **eplus**, **epmfs**, **eprand**) are `path preserving`. `ep` stands for `existing path`.

A path preserving policy will only consider upstreams where the relative path being accessed already exists.

When using non-path preserving policies paths will be created in target upstreams as necessary.

### Quota Relevant Policies

Some policies rely on quota information. These policies should be used only if your upstreams support the respective quota fields.

| Policy     | Required Field |
|------------|----------------|
| lfs, eplfs | Free           |
| mfs, epmfs | Free           |
| lus, eplus | Used           |
| lno, eplno | Objects        |

To check if your upstream supports the field, run `rclone about remote: [flags]` and see if the required field exists.

### Filters

Policies basically search upstream remotes and create a list of files / paths for functions to work on. The policy is responsible for filtering and sorting. The policy type defines the sorting but filtering is mostly uniform as described below.

* No **search** policies filter.
* All **action** policies will filter out remotes which are tagged as **read-only**.
* All **create** policies will filter out remotes which are tagged **read-only** or **no-create**.

If all remotes are filtered an error will be returned.

### Policy descriptions

The policies definition are inspired by [trapexit/mergerfs](https://github.com/trapexit/mergerfs) but not exactly the same. Some policy definition could be different due to the much larger latency of remote file systems.

| Policy           | Description                                                |
|------------------|------------------------------------------------------------|
| all | Search category: same as **epall**. Action category: same as **epall**. Create category: act on all upstreams. |
| epall (existing path, all) | Search category: Given this order configured, act on the first one found where the relative path exists. Action category: apply to all found. Create category: act on all upstreams where the relative path exists. |
| epff (existing path, first found) | Act on the first one found, by the time upstreams reply, where the relative path exists. |
| eplfs (existing path, least free space) | Of all the upstreams on which the relative path exists choose the one with the least free space. |
| eplus (existing path, least used space) | Of all the upstreams on which the relative path exists choose the one with the least used space. |
| eplno (existing path, least number of objects) | Of all the upstreams on which the relative path exists choose the one with the least number of objects. |
| epmfs (existing path, most free space) | Of all the upstreams on which the relative path exists choose the one with the most free space. |
| eprand (existing path, random) | Calls **epall** and then randomizes. Returns only one upstream. |
| ff (first found) | Search category: same as **epff**. Action category: same as **epff**. Create category: Act on the first one found by the time upstreams reply. |
| lfs (least free space) | Search category: same as **eplfs**. Action category: same as **eplfs**. Create category: Pick the upstream with the least available free space. |
| lus (least used space) | Search category: same as **eplus**. Action category: same as **eplus**. Create category: Pick the upstream with the least used space. |
| lno (least number of objects) | Search category: same as **eplno**. Action category: same as **eplno**. Create category: Pick the upstream with the least number of objects. |
| mfs (most free space) | Search category: same as **epmfs**. Action category: same as **epmfs**. Create category: Pick the upstream with the most available free space. |
| newest | Pick the file / directory with the largest mtime. |
| rand (random) | Calls **all** and then randomizes. Returns only one upstream. |


### Writeback {#writeback}

The tag `:writeback` on an upstream remote can be used to make a simple cache
system like this:

```
[union]
type = union
action_policy = all
create_policy = all
search_policy = ff
upstreams = /local:writeback remote:dir
```

When files are opened for read, if the file is in `remote:dir` but not `/local`
then rclone will copy the file entirely into `/local` before returning a
reference to the file in `/local`. The copy will be done with the equivalent of
`rclone copy` so will use `--multi-thread-streams` if configured. Any copies
will be logged with an INFO log.

When files are written, they will be written to both `remote:dir` and `/local`.

As many remotes as desired can be added to `upstreams` but there should only be
one `:writeback` tag.

Rclone does not manage the `:writeback` remote in any way other than writing
files back to it. So if you need to expire old files or manage the size then you
will have to do this yourself.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/union/union.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to union (Union merges the contents of several upstream fs).

#### --union-upstreams

List of space separated upstreams.

Can be 'upstreama:test/dir upstreamb:', '"upstreama:test/space:ro dir" upstreamb:', etc.

Properties:

- Config:      upstreams
- Env Var:     RCLONE_UNION_UPSTREAMS
- Type:        string
- Required:    true

#### --union-action-policy

Policy to choose upstream on ACTION category.

Properties:

- Config:      action_policy
- Env Var:     RCLONE_UNION_ACTION_POLICY
- Type:        string
- Default:     "epall"

#### --union-create-policy

Policy to choose upstream on CREATE category.

Properties:

- Config:      create_policy
- Env Var:     RCLONE_UNION_CREATE_POLICY
- Type:        string
- Default:     "epmfs"

#### --union-search-policy

Policy to choose upstream on SEARCH category.

Properties:

- Config:      search_policy
- Env Var:     RCLONE_UNION_SEARCH_POLICY
- Type:        string
- Default:     "ff"

#### --union-cache-time

Cache time of usage and free space (in seconds).

This option is only useful when a path preserving policy is used.

Properties:

- Config:      cache_time
- Env Var:     RCLONE_UNION_CACHE_TIME
- Type:        int
- Default:     120

### Advanced options

Here are the Advanced options specific to union (Union merges the contents of several upstream fs).

#### --union-min-free-space

Minimum viable free space for lfs/eplfs policies.

If a remote has less than this much free space then it won't be
considered for use in lfs or eplfs policies.

Properties:

- Config:      min_free_space
- Env Var:     RCLONE_UNION_MIN_FREE_SPACE
- Type:        SizeSuffix
- Default:     1Gi

#### --union-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_UNION_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Any metadata supported by the underlying remote is read and written.

See the [metadata](/docs/#metadata) docs for more info.

{{< rem autogenerated options stop >}}
