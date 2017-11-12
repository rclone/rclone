---
title: "Cache"
description: "Rclone docs for cache remote"
date: "2017-09-03"
---

<i class="fa fa-archive"></i> Cache
-----------------------------------------

The `cache` remote wraps another existing remote and stores file structure
and its data for long running tasks like `rclone mount`.

To get started you just need to have an existing remote which can be configured
with `cache`.

Here is an example of how to make a remote called `test-cache`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
...
 5 / Cache a remote
   \ "cache"
...
Storage> 5
Remote to cache.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
remote> local:/test
The size of a chunk. Lower value good for slow connections but can affect seamless reading.
Default: 5M
Choose a number from below, or type in your own value
 1 / 1MB
   \ "1m"
 2 / 5 MB
   \ "5M"
 3 / 10 MB
   \ "10M"
chunk_size> 2
How much time should object info (file size, file hashes etc) be stored in cache. Use a very high value if you don't plan on changing the source FS from outside the cache.
Accepted units are: "s", "m", "h".
Default: 5m
Choose a number from below, or type in your own value
 1 / 1 hour
   \ "1h"
 2 / 24 hours
   \ "24h"
 3 / 24 hours
   \ "48h"
info_age> 2
How much time should a chunk (file data) be stored in cache.
Accepted units are: "s", "m", "h".
Default: 3h
Choose a number from below, or type in your own value
 1 / 30 seconds
   \ "30s"
 2 / 1 minute
   \ "1m"
 3 / 1 hour and 30 minutes
   \ "1h30m"
chunk_age> 3h
How much time should data be cached during warm up.
Accepted units are: "s", "m", "h".
Default: 24h
Choose a number from below, or type in your own value
 1 / 3 hours
   \ "3h"
 2 / 6 hours
   \ "6h"
 3 / 24 hours
   \ "24h"
warmup_age> 3
Remote config
--------------------
[test-cache]
remote = local:/test
chunk_size = 5M
info_age = 24h
chunk_age = 3h
warmup_age = 24h
```

You can then use it like this,

List directories in top level of your drive

    rclone lsd test-cache:

List all the files in your drive

    rclone ls test-cache:

To start a cached mount

    rclone mount --allow-other test-cache: /var/tmp/test-cache

### Write Support ###

Writes are supported through `cache`.
One caveat is that a mounted cache remote does not add any retry or fallback
mechanism to the upload operation. This will depend on the implementation
of the wrapped remote.

One special case is covered with `cache-writes` which will cache the file
data at the same time as the upload when it is enabled making it available
from the cache store immediately once the upload is finished.

### Read Features ###

#### Multiple connections ####

To counter the high latency between a local PC where rclone is running
and cloud providers, the cache remote can split multiple requests to the
cloud provider for smaller file chunks and combines them together locally
where they can be available almost immediately before the reader usually
needs them.
This is similar to buffering when media files are played online. Rclone
will stay around the current marker but always try its best to stay ahead
and prepare the data before.

#### Warmup mode ####

A negative side of running multiple requests on the cloud provider is
that you can easily reach a limit on how many requests or how much data
you can download from a cloud provider in a window of time.
For this reason, a warmup mode is a state where `cache` changes its settings
to talk less with the cloud provider.

To prevent a ban or a similar action from the cloud provider, `cache` will
keep track of all the open files and during intensive times when it passes
a configured threshold, it will change its settings to a warmup mode.

It can also be disabled during single file streaming if `cache` sees that we're
reading the file in sequence and can go load its parts in advance.

Affected settings:
- `cache-chunk-no-memory`: _disabled_
- `cache-workers`: _1_
- file chunks will now be cached using `cache-warm-up-age` as a duration instead of the
regular `cache-chunk-age`

### Known issues ###

#### cache and crypt ####

One common scenario is to keep your data encrypted in the cloud provider
using the `crypt` remote. `crypt` uses a similar technique to wrap around
an existing remote and handles this translation in a seamless way.

There is an issue with wrapping the remotes in this order:
<span style="color:red">**cloud remote** -> **crypt** -> **cache**</span>

During testing, I experienced a lot of bans with the remotes in this order.
I suspect it might be related to how crypt opens files on the cloud provider
which makes it think we're downloading the full file instead of small chunks.
Organizing the remotes in this order yelds better results:
<span style="color:green">**cloud remote** -> **cache** -> **crypt**</span>

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --cache-db-path=PATH ####

Path to where partial file data (chunks) and the file structure metadata 
are stored locally.

**Default**: <rclone default config path>/<remote name>
**Example**: ~/.config/rclone/test-cache

#### --cache-db-purge ####

Flag to clear all the cached data for this remote before.

**Default**: not set

#### --cache-chunk-size=SIZE ####

The size of a chunk (partial file data). Use lower numbers for slower
connections.

**Default**: 5M

#### --cache-info-age=DURATION ####

How long to keep file structure information (directory listings, file size, 
mod times etc) locally. 

If all write operations are done through `cache` then you can safely make
this value very large as the cache store will also be updated in real time.

**Default**: 6h

#### --cache-chunk-age=DURATION ####

How long to keep file chunks (partial data) locally. 

Longer durations will result in larger cache stores as data will be cleared
less often.

**Default**: 3h

#### --cache-warm-up-age=DURATION ####

How long to keep file chunks (partial data) locally during warmup times.

If `cache` goes through intensive read times when it is scanned for information
then this setting will allow you to customize higher storage times for that
data. Otherwise, it's safe to keep the same value as `cache-chunk-age`.

**Default**: 3h

#### --cache-read-retries=RETRIES ####

How many times to retry a read from a cache storage.

Since reading from a `cache` stream is independent from downloading file data, 
readers can get to a point where there's no more data in the cache. 
Most of the times this can indicate a connectivity issue if `cache` isn't
able to provide file data anymore.

For really slow connections, increase this to a point where the stream is
able to provide data but your experience will be very stuttering. 

**Default**: 3

#### --cache-workers=WORKERS ####

How many workers should run in parallel to download chunks.

Higher values will mean more parallel processing (better CPU needed) and
more concurrent requests on the cloud provider. 
This impacts several aspects like the cloud provider API limits, more stress
on the hardware that rclone runs on but it also means that streams will 
be more fluid and data will be available much more faster to readers.

**Default**: 4

#### --cache-chunk-no-memory ####

By default, `cache` will keep file data during streaming in RAM as well
to provide it to readers as fast as possible.

This transient data is evicted as soon as it is read and the number of
chunks stored doesn't exceed the number of workers. However, depending
on other settings like `cache-chunk-size` and `cache-workers` this footprint
can increase if there are parallel streams too (multiple files being read
at the same time).

If the hardware permits it, use this feature to provide an overall better
performance during streaming but it can also be disabled if RAM is not
available on the local machine.

**Default**: not set

#### --cache-rps=NUMBER ####

Some of the rclone remotes that `cache` will wrap have back-off or limits
in place to not reach cloud provider limits. This is similar to that.
It places a hard limit on the number of requests per second that `cache`
will be doing to the cloud provider remote and try to respect that value
by setting waits between reads.

If you find that you're getting banned or limited on the cloud provider
through cache and know that a smaller number of requests per second will
allow you to work with it then you can use this setting for that.

A good balance of all the other settings and warmup times should make this
setting useless but it is available to set for more special cases.

**NOTE**: This will limit the number of requests during streams but other
API calls to the cloud provider like directory listings will still pass.

**Default**: 4

#### --cache-warm-up-rps=RATE/SECONDS ####

This setting allows `cache` to change its settings for warmup mode or revert
back from it.

`cache` keeps track of all open files and when there are `RATE` files open
during `SECONDS` window of time reached it will activate warmup and change
its settings as explained in the `Warmup mode` section.

When the number of files being open goes under `RATE` in the same amount
of time, `cache` will disable this mode.

**Default**: 3/20

#### --cache-writes ####

If you need to read files immediately after you upload them through `cache`
you can enable this flag to have their data stored in the cache store at the
same time during upload.

**Default**: not set
