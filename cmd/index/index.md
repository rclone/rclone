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

### Partial runs

After a sync which changed a known set of files there is no need to
walk the whole remote. Tell `rclone index` what changed and it re-indexes
only the directories that could be affected:

    rclone index r2:bucket --changed v1.76.0/ --changed version.txt
    rclone index r2:bucket --changed-from changes.txt
    rclone sync ./public r2:bucket --exclude index.html --combined - | rclone index r2:bucket --changed-combined -

- `--changed PATH` names a changed file or directory relative to
  `remote:path` and can be repeated. A directory, given with a trailing
  slash or found to be one, is re-indexed completely.
- `--changed-from FILE` reads one path per line exactly as written.
- `--changed-combined FILE` reads the `--combined` report of
  `rclone sync`: lines starting with `+`, `-`, `*` and `!` are changes and
  `=` lines are ignored.
- Both files accept `-` for standard input and all three flags can be
  combined.

A changed file re-indexes its directory and every directory above it,
since their times and their listings can change too, and on bucket
based storage a directory can cease to exist. Each of those is one
directory listing. With `--dir-time newest` the directories above also
need the times of their unchanged subdirectories, which are read from
the modification times of those subdirectories' listings at the cost
of one small request each. With `--use-server-modtime` that is the time
the listing was written rather than the time of the newest file, so
partial runs can drift by the indexing delay until the next full run.
`--dir-time dir` and `none` need no such lookups.

Directories which are neither named nor above a named path are left
alone, as are directories deeper than `--max-depth`. An empty change
list does nothing, and naming the root does a full run. There is no
automatic cutover to a full run since that depends on the size of the
whole remote: a partial run costs one listing per affected directory,
a full run one request per thousand objects on S3-like storage or one
per directory elsewhere. The number
of directories a partial run will list is logged with `-v`, and
`--changed-max-dirs` makes it fall back to a full run above that
number. A good pattern is a partial run after each upload and a
scheduled full run to catch anything missed.

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
