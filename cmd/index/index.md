Write a directory listing, `index.html` by default, into every
directory of `remote:path` so that it can be browsed when it is served
as a static website, for example from an S3 website endpoint, a bucket
behind a CDN, or a plain web server.

This works like `rclone sync`: each run makes the minimum changes needed
to bring the listings into line with the files. Listings which haven't
changed aren't rewritten, so a run with no changes makes no uploads. The
listings never show the listing files themselves.

**Note** that `rclone index` treats every file with an output name
(`index.html` by default) in the directories it indexes as its own, and
will overwrite it or delete it without checking where it came from. To
protect a hand-written index, exclude its directory with
`--index-exclude`.

### Outputs

`--output NAME=FORMAT` says what to write in each directory and can be
repeated. NAME is the file name, and FORMAT is one of the built-in
formats below or the path of a Go template file.

| Format | MIME type | Contents |
|:-------|:----------|:---------|
| `html` | `text/html; charset=utf-8` | The HTML listing from the `rclone serve http` template |
| `json` | `application/json` | JSON in the format of `rclone lsjson` for the directory |
| `caddy` | `application/json` | JSON in the format of Caddy's `file_server browse` |

The default is `--output index.html=html`. The built-in formats are
templates, so they can be printed with `--print-template FORMAT`, copied
and changed. Outputs whose name ends in `.html` or `.htm` are rendered
with Go's `html/template` and everything else with `text/template`, which
has a `json` function for safe encoding. The template data is that of
`rclone serve http` with `.Static` set (see
[rclone serve http](/commands/rclone_serve_http/#template)). Outputs are
uploaded with the MIME type of the built-in format, or that of the
output name's extension for template outputs, since static hosts serve
the stored type.

### Filters

The normal filters (`--include`, `--exclude`, `--filter-from`, `--max-age`, ...)
control what appears in the listings. An excluded file isn't listed, and
an excluded directory isn't listed or indexed.

The `--index-include`, `--index-exclude`, `--index-filter` flags and their
`-from` variants control which directories get listings. They take the same
patterns as the normal filters, applied to directory paths, and
`--index-include` implies excluding all other directories. A directory
excluded here still appears in its parent's listing, but nothing is written
or deleted inside it, so it can have its own hand-made index.
`--index-max-depth` limits how deep listings are written.

### Modification times

Listings show the modification time of each file. When using the s3,
oracleobjectstorage and swift backends rclone stores the modification time as
metadata which isn't returned by the bucket listing, so **every object needs
a HEAD request**, which can mean thousands of transactions per run. Avoid
this with one of:

- `--use-server-modtime` uses the time the object was uploaded instead,
  which is what the web server sends as `Last-Modified`. Recommended for s3
  unless the original modification times matter.
- `--no-modtime` shows no times at all, which is the cheapest option.

`--dir-time` sets the time shown for directories:

- `newest` (default): the newest modification time of anything below the
  directory. Note that this means one new file changes the listing of
  every directory above it.
- `dir`: the directory's own modification time. On most file systems
  writing a listing into a directory changes its time, so this can take
  more than one run to settle.
- `none`: no time.

The listing files are given the newest modification time below their
directory, so the `Last-Modified` header of a listing is meaningful.

### Links

Directory links are `dir/` by default, which works on hosts that serve
index documents: S3 website endpoints, GCS and Azure static websites,
nginx and Apache, and R2 custom domains with a URL rewrite rule. For
hosts that don't (plain S3 REST URLs, B2 friendly URLs) use
`--link-index` to make directory links `dir/index.html` instead.

### Site icon

The listings carry no icon or branding of their own. To give a site an
icon, upload a `favicon.ico` to the root of the remote, which browsers
fetch from there for every page. Exclude it from the listings with
`--exclude /favicon.ico` so it stays on the site but isn't shown.

### Deleting listings

On backends which can't have empty directories (S3, B2 and other
bucket-based storage) a directory only exists while it has files in it.
Once the last file in a directory has been removed its listing is
deleted, since the directory only existed because of it. Backends which
can have empty directories keep a listing for each empty directory.

### Using with sync

A sync from a local build deletes files on the destination which aren't
in the source, which includes the listings. Either index the local build
before syncing:

    rclone index ./public
    rclone sync ./public r2:bucket

or exclude the listings from the sync so it neither uploads nor deletes
them, then index the remote:

    rclone sync ./public r2:bucket --exclude index.html
    rclone index r2:bucket

### Examples

An S3 website bucket, using the upload times so no object needs a HEAD
request:

    rclone index s3:my-site --use-server-modtime

HTML and rclone JSON listings, setting `Cache-Control` on them:

    rclone index s3:bucket --output index.html=html --output index.json=json --header-upload "Cache-Control: max-age=300"

Only list release files, and don't write listings under `/private`:

    rclone index r2:bucket --include "*.{zip,deb,rpm}" --index-exclude "/private/**"

A B2 bucket served from its friendly URLs:

    rclone index b2:bucket --link-index

See what would change:

    rclone index r2:bucket --dry-run -v

Listings which haven't changed aren't normally rewritten, so changing
the headers or metadata set on them needs `--index-rewrite`, which
writes every listing a run would render whether it changed or not:

    rclone index r2:bucket --index-rewrite --header-upload "Cache-Control: max-age=3600"
