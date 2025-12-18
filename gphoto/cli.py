import argparse
from pprint import pp

from .client import Client
from .api import DEFAULT_TIMEOUT


def main():
    parser = argparse.ArgumentParser(description="Google Photos mobile client.", formatter_class=argparse.RawTextHelpFormatter)
    parser.add_argument("path", type=str, help="Path to the file or directory to upload.")
    parser.add_argument("--auth_data", type=str, help="Google auth data for authentication. If not provided, `GP_AUTH_DATA` env variable will be used.")
    parser.add_argument(
        "--album",
        type=str,
        help=(
            "Add uploaded media to an album with given name. If set to 'AUTO', albums will be created based on the immediate parent directory of each file.\n"
            "Example for using 'AUTO':\n"
            "When uploading '/foo':\n"
            "'/foo/image1.jpg' goes to 'foo'\n"
            "'/foo/bar/image2.jpg' goes to 'bar'\n"
            "'/foo/bar/foo/image3.jpg' goes to 'foo' (distinct from the first 'foo' album)\n"
        ),
    )
    parser.add_argument("--proxy", type=str, help="Proxy to use. Format: `protocol://username:password@host:port`")
    parser.add_argument("--progress", action="store_true", help="Display upload progress.")
    parser.add_argument("--recursive", action="store_true", help="Scan the directory recursively.")
    parser.add_argument("--threads", type=int, default=1, help="Number of threads to run uploads with. Defaults to 1.")
    parser.add_argument("--force-upload", action="store_true", help="Upload files regardless of their presence in Google Photos (determined by hash).")
    parser.add_argument("--delete-from-host", action="store_true", help="Delete uploaded files from source path.")
    parser.add_argument("--use-quota", action="store_true", help="Uploaded files will count against your Google Photos storage quota.")
    parser.add_argument("--saver", action="store_true", help="Upload files in storage saver quality.")
    parser.add_argument("--timeout", type=int, default=30, help=f"Requests timeout, seconds. Defaults to {DEFAULT_TIMEOUT}.")
    parser.add_argument("--log-level", type=str, default="INFO", choices=["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"], help="Set the logging level (default: INFO)")

    filter_group = parser.add_argument_group("File Filter Options")
    filter_group.add_argument("--filter", type=str, help="Filter expression.")
    filter_group.add_argument("--exclude", action="store_true", help="Exclude files matching the filter.")
    filter_group.add_argument("--regex", action="store_true", help="Use regex for filtering.")
    filter_group.add_argument("--ignore-case", action="store_true", help="Perform case-insensitive matching.")
    filter_group.add_argument("--match-path", action="store_true", help="Check for matches in the path, not just the filename.")

    args = parser.parse_args()

    if (args.exclude or args.regex or args.ignore_case or args.match_path) and not args.filter:
        parser.error("--filter is required when using any of --exclude, --regex, --ignore-case, or --match-path")

    client = Client(
        auth_data=args.auth_data,
        timeout=args.timeout,
        log_level=args.log_level,
        proxy=args.proxy,
    )
    output = client.upload(
        target=args.path,
        album_name=args.album,
        use_quota=args.use_quota,
        saver=args.saver,
        show_progress=args.progress,
        recursive=args.recursive,
        threads=args.threads,
        force_upload=args.force_upload,
        delete_from_host=args.delete_from_host,
        filter_exp=args.filter,
        filter_exclude=args.exclude,
        filter_regex=args.regex,
        filter_ignore_case=args.ignore_case,
        filter_path=args.match_path,
    )
    pp(output)
