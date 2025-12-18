import mimetypes
import os
import re
from pathlib import Path
from typing import Mapping, Sequence

import aiofiles
from httpx import HTTPStatusError
from rich.progress import (
    DownloadColumn,
    MofNCompleteColumn,
    Progress,
    SpinnerColumn,
    TaskID,
    TaskProgressColumn,
    TextColumn,
    TimeElapsedColumn,
    TimeRemainingColumn,
    TransferSpeedColumn,
)

from utils.live import DynamicLive
from utils.logger2 import logger, shutdown_event

from . import utils
from .api import DEFAULT_TIMEOUT, Api
from .db import AsyncStorage
from .db_update_parser import parse_db_update
from .hash_handler import calculate_sha1_hash, convert_sha1_hash


class Client:
    """Google Photos client based on reverse engineered mobile API."""

    def __init__(
        self,
        live: DynamicLive,
        proxy: str = "",
        language: str = "",
        timeout: int = DEFAULT_TIMEOUT,
    ) -> None:
        """
        Google Photos client based on reverse engineered mobile API.

        Args:
            auth_data: Google authentication data string. If not provided, will attempt to use
                      the `GP_AUTH_DATA` environment variable.
            proxy: Proxy url `protocol://username:password@ip:port`.
            language: Accept-Language header value. If not provided, will attempt to parse from auth_data. Fallback value is `en_US`.
            log_level: Logging level to use. Must be one of "INFO", "DEBUG", "WARNING",
                      "ERROR", or "CRITICAL". Defaults to "INFO".
            timeout: Requests timeout, seconds. Defaults to DEFAULT_TIMEOUT.

        Raises:
            ValueError: If no auth_data is provided and GP_AUTH_DATA environment variable is not set.
            requests.HTTPError: If the authentication request fails.
        """
        self.logger = logger
        self.live = live
        self.valid_mimetypes = ["image/", "video/"]
        self.timeout = timeout

        self.language = "en_US"

        self.api = Api(proxy=proxy, language=self.language, timeout=timeout)
        self.dsn = "postgresql://vavtnen:637578@localhost:10012/gphotos"
        self.cache_dir = Path.home() / ".gpmc"
        self.db_path = self.cache_dir / "storage.db"

    async def _file_stream(
        self, file_path: Path, progress: Progress, task_id: TaskID, chunk_size=65536
    ):
        async with aiofiles.open(file_path, "rb") as f:
            while True:
                chunk = await f.read(chunk_size)
                if not chunk:
                    break
                progress.update(task_id, advance=len(chunk))
                yield chunk

    async def _upload_file(
        self,
        file_path: str | Path,
        hash_value: bytes | str,
        progress: Progress,
        force_upload: bool,
        use_quota: bool,
        saver: bool,
    ) -> dict[str, str]:
        """
        Upload a single file to Google Photos.

        Args:
            file_path: Path to the file to upload, can be string or Path object.
            hash_value: The file's SHA-1 hash, represented as bytes, a hexadecimal string,
                    or a Base64-encoded string.
            progress: Rich Progress object for tracking upload progress.
            force_upload: Whether to upload the file even if it's already present in Google Photos.
            use_quota: Uploaded files will count against your Google Photos storage quota.
            saver: Upload files in storage saver quality.

        Returns:
            dict[str, str]: A dictionary mapping the absolute file path to its Google Photos media key.

        Raises:
            FileNotFoundError: If the file does not exist.
            IOError: If there are issues reading the file.
            ValueError: If the file is empty or cannot be processed.
        """

        file_path = Path(file_path)
        file_size = file_path.stat().st_size

        file_progress_id = progress.add_task(description="")
        if hash_value:
            hash_bytes, hash_b64 = convert_sha1_hash(hash_value)
        else:
            hash_bytes, hash_b64 = await calculate_sha1_hash(
                file_path, progress, file_progress_id
            )
        try:
            if not force_upload:
                progress.update(
                    task_id=file_progress_id,
                    description=f"Checking: {file_path.name}"[:60],
                )
                if remote_media_key := await self.api.find_remote_media_by_hash(
                    hash_bytes
                ):
                    return {file_path.absolute().as_posix(): remote_media_key}

            upload_token = await self.api.get_upload_token(hash_b64, file_size)
            progress.reset(task_id=file_progress_id)
            progress.update(
                task_id=file_progress_id,
                description=f"Uploading: {file_path.name}"[:60],
            )
          
            upload_response = await self.api.upload_file(
                file=self._file_stream(file_path, progress, file_progress_id),
                upload_token=upload_token,
            )

            progress.update(
                task_id=file_progress_id,
                description=f"Commit Upload: {file_path.name}"[:60],
            )
            last_modified_timestamp = int(os.path.getmtime(file_path))
            model = "Pixel XL"
            quality = "original"
            if saver:
                quality = "saver"
                model = "Pixel 2"
            if use_quota:
                model = "Pixel 8"
            media_key = await self.api.commit_upload(
                upload_response_decoded=upload_response,
                file_name=file_path.name,
                sha1_hash=hash_bytes,
                upload_timestamp=last_modified_timestamp,
                model=model,
                quality=quality,
            )
            return {file_path.absolute().as_posix(): media_key}
        finally:
            progress.update(file_progress_id, visible=False)

    async def get_media_key_by_hash(self, sha1_hash: bytes | str) -> str | None:
        """
        Get a Google Photos media key by media's hash.

        Args:
            sha1_hash: The file's SHA-1 hash, represented as bytes, a hexadecimal string,
                    or a Base64-encoded string.

        Returns:
            str | None: The Google Photos media key if found, otherwise None.
        """
        hash_bytes, _ = convert_sha1_hash(sha1_hash)
        return await self.api.find_remote_media_by_hash(
            hash_bytes,
        )

    async def _handle_album_creation(
        self, results: dict[str, str], album_name: str, show_progress: bool
    ) -> None:
        """
        Handle album creation based on the provided album_name.

        Args:
            results: Dictionary mapping file paths to their Google Photos media keys.
            album_name: Name of album to create. "AUTO" creates albums based on parent directories.
            show_progress: Whether to display progress in the console.
        """
        if album_name != "AUTO":
            # Add all media keys to the specified album
            media_keys = list(results.values())
            await self.add_to_album(media_keys, album_name, show_progress=show_progress)
            return

        # Group media keys by the full path of their parent directory
        media_keys_by_album = {}
        for file_path, media_key in results.items():
            parent_dir = Path(file_path).parent.resolve().as_posix()
            if parent_dir not in media_keys_by_album:
                media_keys_by_album[parent_dir] = []
            media_keys_by_album[parent_dir].append(media_key)

        for parent_dir, media_keys in media_keys_by_album.items():
            album_name_from_path = Path(
                parent_dir
            ).name  # Use the directory name as the album name
            await self.add_to_album(
                media_keys, album_name_from_path, show_progress=show_progress
            )

    @staticmethod
    def _filter_files(
        expression: str,
        filter_exclude: bool,
        filter_regex: bool,
        filter_ignore_case: bool,
        filter_path: bool,
        paths: list[Path],
    ) -> list[Path]:
        """
        Filter a list of Path objects based on a filter expression.

        Args:
            expression: The filter expression to match against.
            filter_exclude: If True, exclude matching files.
            filter_regex: If True, treat expression as regex.
            filter_ignore_case: If True, perform case-insensitive matching.
            filter_path: If True, check full path instead of just filename.
            paths: List of Path objects to filter.

        Returns:
            list[Path]: Filtered list of Path objects.
        """
        filtered_paths = []

        for path in paths:
            text_to_check = str(path) if filter_path else str(path.name)

            if filter_regex:
                flags = re.IGNORECASE if filter_ignore_case else 0
                matches = bool(re.search(expression, text_to_check, flags))
            else:
                if filter_ignore_case:
                    matches = expression.lower() in text_to_check.lower()
                else:
                    matches = expression in text_to_check

            if (matches and not filter_exclude) or (not matches and filter_exclude):
                filtered_paths.append(path)

        return filtered_paths

    async def upload(
        self,
        target: str | Path | Sequence[str | Path] | Mapping[Path, bytes | str],
        album_name: str | None = None,
        use_quota: bool = False,
        saver: bool = False,
        recursive: bool = False,
        show_progress: bool = False,
        threads: int = 1,
        force_upload: bool = False,
        delete_from_host: bool = False,
        filter_exp: str = "",
        filter_exclude: bool = False,
        filter_regex: bool = False,
        filter_ignore_case: bool = False,
        filter_path: bool = False,
    ) -> dict[str, str]:
        """
        Upload one or more files or directories to Google Photos.

        Args:
            target: A file path, directory path, a sequence of such paths, or a mapping of file paths to their SHA-1 hashes.
            album_name:
                If provided, the uploaded media will be added to a new album.
                If set to "AUTO", albums will be created based on the immediate parent directory of each file.

                "AUTO" Example:
                    - When uploading '/foo':
                        - '/foo/image1.jpg' will be placed in a 'foo' album.
                        - '/foo/bar/image2.jpg' will be placed in a 'bar' album.
                        - '/foo/bar/foo/image3.jpg' will be placed in a 'foo' album, distinct from the first 'foo' album.

                Defaults to None.
            use_quota: Uploaded files will count against your Google Photos storage quota. Defaults to False.
            saver: Upload files in storage saver quality. Defaults to False.
            recursive: Whether to recursively search for media files in subdirectories.
                                Only applies when uploading directories. Defaults to False.
            show_progress: Whether to display upload progress in the console. Defaults to False.
            threads: Number of concurrent upload threads for multiple files. Defaults to 1.
            force_upload: Whether to upload files even if they're already present in
                                Google Photos (based on hash). Defaults to False.
            delete_from_host: Whether to delete the file from the host after successful upload.
                                    Defaults to False.
            filter_exp: The filter expression to match against filenames or paths.
            filter_exclude: If True, exclude files matching the filter.
            filter_regex: If True, treat the expression as a regular expression.
            filter_ignore_case: If True, perform case-insensitive matching.
            filter_path: If True, check for matches in the full path instead of just the filename.

        Returns:
            dict[str, str]: A dictionary mapping absolute file paths to their Google Photos media keys.
                            Example: {
                                "/path/to/photo1.jpg": "media_key_123",
                                "/path/to/photo2.jpg": "media_key_456"
                            }

        Raises:
            TypeError: If `target` is not a file path, directory path, or a squence of such paths.
            ValueError: If no valid media files are found to upload.
        """

        path_hash_pairs = self._handle_target_input(
            target,
            recursive,
            filter_exp,
            filter_exclude,
            filter_regex,
            filter_ignore_case,
            filter_path,
        )

        results = await self._upload_concurrently(
            path_hash_pairs,
            threads=threads,
            show_progress=show_progress,
            force_upload=force_upload,
            use_quota=use_quota,
            saver=saver,
        )

        if album_name:
            await self._handle_album_creation(results, album_name, show_progress)

        if delete_from_host:
            for file_path, _ in results.items():
                self.logger.info(f"{file_path} deleting from host")
                os.remove(file_path)
        return results

    def _handle_target_input(
        self,
        target: str | Path | Sequence[str | Path] | Mapping[Path, bytes | str],
        recursive: bool,
        filter_exp: str,
        filter_exclude: bool,
        filter_regex: bool,
        filter_ignore_case: bool,
        filter_path: bool,
    ) -> Mapping[Path, bytes | str]:
        """
        Process and validate the upload target input into a consistent path-hash mapping.

        Args:
            target: A file path, directory path, sequence of paths, or mapping of paths to hashes.
            recursive: Whether to search directories recursively for media files.
            filter_exp: The filter expression to match against filenames or paths.
            filter_exclude: If True, exclude files matching the filter.
            filter_regex: If True, treat the expression as a regular expression.
            filter_ignore_case: If True, perform case-insensitive matching.
            filter_path: If True, check for matches in the full path instead of just the filename.

        Returns:
            Mapping[Path, bytes | str]: A dictionary mapping file paths to their SHA-1 hashes.
                                    Files without precomputed hashes will have empty bytes (b"").

        Raises:
            TypeError: If `target` is not a valid path, sequence of paths, or path-to-hash mapping.
            ValueError: If no valid media files are found or if filtering leaves no files to upload.
        """
        path_hash_pairs: Mapping[Path, bytes | str] = {}
        if isinstance(target, (str, Path)):
            target = [target]

            if not isinstance(target, Sequence) or not all(
                isinstance(p, (str, Path)) for p in target
            ):
                raise TypeError(
                    "`target` must be a file path, a directory path, or a squence of such paths."
                )

            # Expand all paths to a flat list of files
            files_to_upload = [
                file
                for path in target
                for file in self._search_for_media_files(path, recursive=recursive)
            ]

            if not files_to_upload:
                raise ValueError("No valid media files found to upload.")

            if filter_exp:
                files_to_upload = self._filter_files(
                    filter_exp,
                    filter_exclude,
                    filter_regex,
                    filter_ignore_case,
                    filter_path,
                    files_to_upload,
                )

            if not files_to_upload:
                raise ValueError("No media files left after filtering.")

            for path in files_to_upload:
                path_hash_pairs[path] = b""  # epmty hash values to be calculated later

        elif isinstance(target, dict) and all(
            isinstance(k, Path) and isinstance(v, (bytes, str))
            for k, v in target.items()
        ):
            path_hash_pairs = target
        return path_hash_pairs

    def _search_for_media_files(self, path: str | Path, recursive: bool) -> list[Path]:
        """
        Search for valid media files in the specified path.

        Args:
            path: File or directory path to search for media files.
            recursive: Whether to search subdirectories recursively. Only applies
                             when path is a directory.

        Returns:
            list[Path]: List of Path objects pointing to valid media files.

        Raises:
            ValueError: If the path is invalid, or if no valid media files are found,
                       or if a single file's mime type is not supported.
        """
        path = Path(path)

        if path.is_file():
            if any(
                mimetype_guess is not None and mimetype_guess.startswith(mimetype)
                for mimetype in self.valid_mimetypes
                if (mimetype_guess := mimetypes.guess_type(path)[0])
            ):
                return [path]
            raise ValueError(
                "File's mime type does not match image or video mime type."
            )

        if not path.is_dir():
            raise ValueError("Invalid path. Please provide a file or directory path.")

        files = []
        if recursive:
            for root, _, filenames in os.walk(path):
                for filename in filenames:
                    file_path = Path(root) / filename
                    files.append(file_path)
        else:
            files = [file for file in path.iterdir() if file.is_file()]

        if len(files) == 0:
            raise ValueError("No files in the directory.")

        media_files = [
            file
            for file in files
            if any(
                mimetype_guess is not None and mimetype_guess.startswith(mimetype)
                for mimetype in self.valid_mimetypes
                if (mimetype_guess := mimetypes.guess_type(file)[0]) is not None
            )
        ]

        if len(media_files) == 0:
            raise ValueError(
                "No files in the directory matched image or video mime types"
            )

        return media_files

    async def _calculate_hash(
        self, file_path: Path, progress: Progress
    ) -> tuple[Path, bytes]:
        hash_calc_progress_id = progress.add_task(description="Calculating hash")
        try:
            hash_bytes, _ = await calculate_sha1_hash(
                file_path, progress, hash_calc_progress_id
            )
            return file_path, hash_bytes
        finally:
            progress.update(hash_calc_progress_id, visible=False)

    async def _upload_concurrently(
        self,
        path_hash_pairs: Mapping[Path, bytes | str],
        threads: int,
        show_progress: bool,
        force_upload: bool,
        use_quota: bool,
        saver: bool,
    ) -> dict[str, str]:
        """
        Upload files concurrently to Google Photos.

        Args:
            path_hash_pairs: Mapping of file paths to their SHA-1 hashes.
            threads: Number of concurrent upload threads.
            show_progress: Whether to display progress in console.
            force_upload: Upload even if file exists in Google Photos.
            use_quota: Count uploads against storage quota.
            saver: Upload in storage saver quality.

        Returns:
            dict[str, str]: Dictionary mapping file paths to media keys.

        Note:
            Failed uploads are logged but don't stop the overall process.
        """
        uploaded_files = {}
        upload_error_count = 0
        overall_progress_columns = [
            TextColumn("[bold yellow]Files processed:"),
            SpinnerColumn(),
            MofNCompleteColumn(),
            TimeElapsedColumn(),
            TextColumn("{task.description}"),
        ]
        file_progress_columns = [
            DownloadColumn(),
            TaskProgressColumn(),
            TimeRemainingColumn(),
            TransferSpeedColumn(),
            TextColumn("{task.description}"),
        ]
        overall_progress = await self.live.add_progress(
            "overall", columns=overall_progress_columns, title="Progress"
        )

        file_progress = await self.live.add_progress(
            "uploader", columns=file_progress_columns, title="Uploader"
        )

        # progress_group = Group(
        #     file_progress,
        #     overall_progress,
        # )
        # empty = Progress()

        # live.add(file_progress)
        # live.add(overall_progress)
        # live.add(empty)

        # context = (show_progress and Live(progress_group)) or nullcontext()

        overall_task_id = overall_progress.add_task(
            "Errors: 0", total=len(path_hash_pairs.keys()), visible=show_progress
        )
        # with context:
        for path, hash_value in path_hash_pairs.items():
            if shutdown_event.is_set():
                return uploaded_files

            try:
                media_key_dict = await self._upload_file(
                    path,
                    hash_value,
                    progress=file_progress,
                    force_upload=force_upload,
                    use_quota=use_quota,
                    saver=saver,
                )

                self.logger.info(media_key_dict)
                uploaded_files = uploaded_files | media_key_dict
                os.remove(path)
            except HTTPStatusError as e:
                self.logger.error(f"HTTP error occurred {path}: {e}")
                if e.response.status_code == 401:
                    self.api.token = await self.api.get_auth_token()
                # else:
                #     print(f"⚠️ HTTP error occurred: {e.response.status_code}")

                upload_error_count += 1
                overall_progress.update(
                    task_id=overall_task_id,
                    description=f"[bold red] Errors: {upload_error_count}",
                )

            # except Exception as e:
            #     self.logger.error(f"Error uploading file {path}: {e}")
            #     upload_error_count += 1
            #     overall_progress.update(
            #         task_id=overall_task_id,
            #         description=f"[bold red] Errors: {upload_error_count}",
            #     )
            finally:
                overall_progress.advance(overall_task_id)

        # with context:
        # with ThreadPoolExecutor(max_workers=threads) as executor:
        #     futures = {
        #         executor.submit(
        #             await self._upload_file,
        #             path,
        #             hash_value,
        #             progress=file_progress,
        #             force_upload=force_upload,
        #             use_quota=use_quota,
        #             saver=saver,
        #         ): (path, hash_value)
        #         for path, hash_value in path_hash_pairs.items()
        #     }
        #     for future in as_completed(futures):
        #         file = futures[future]
        #         try:
        #             media_key_dict = future.result()
        #             self.logger.info(media_key_dict)
        #             uploaded_files = uploaded_files | media_key_dict
        #             os.remove(file[0])

        #         except HTTPStatusError as e:
        #             self.logger.error(f"HTTP error occurred {file}: {e}")
        #             if e.response.status_code == 401:
        #                 self.api.token = await self.api.get_auth_token()
        #             # else:
        #             #     print(f"⚠️ HTTP error occurred: {e.response.status_code}")

        #             upload_error_count += 1
        #             overall_progress.update(
        #                 task_id=overall_task_id,
        #                 description=f"[bold red] Errors: {upload_error_count}",
        #             )

        #         except Exception as e:
        #             self.logger.error(f"Error uploading file {file}: {e}")
        #             upload_error_count += 1
        #             overall_progress.update(
        #                 task_id=overall_task_id,
        #                 description=f"[bold red] Errors: {upload_error_count}",
        #             )
        #         finally:
        #             overall_progress.advance(overall_task_id)
        return uploaded_files

    async def move_to_trash(
        self, sha1_hashes: str | bytes | Sequence[str | bytes]
    ) -> dict:
        """
        Move remote media files to trash.

        Args:
            sha1_hashes: Single SHA-1 hash or sequence of hashes to move to trash.

        Returns:
            dict: API response containing operation results.

        Raises:
            ValueError: If input hashes are invalid.
        """

        if isinstance(sha1_hashes, (str, bytes)):
            sha1_hashes = [sha1_hashes]

        try:
            # Convert all hashes to Base64 format
            hashes_b64 = [convert_sha1_hash(hash)[1] for hash in sha1_hashes]  # type: ignore
            dedup_keys = [utils.urlsafe_base64(hash) for hash in hashes_b64]
        except (TypeError, ValueError) as e:
            raise ValueError("Invalid SHA-1 hash format") from e

        # Process in batches of 500 to avoid API limits
        batch_size = 500
        response = {}
        for i in range(0, len(dedup_keys), batch_size):
            batch = dedup_keys[i : i + batch_size]
            batch_response = await self.api.move_remote_media_to_trash(dedup_keys=batch)
            response.update(batch_response)  # Combine responses if needed

        return response

    async def add_to_album(
        self, media_keys: Sequence[str], album_name: str, show_progress: bool
    ) -> list[str]:
        """
        Add media items to one or more albums with the given name. If the total number of items exceeds the album limit,
        additional albums with numbered suffixes are created. The first album will also have a suffix if there are multiple albums.

        Args:
            media_keys: Media keys of the media items to be added to album.
            album_name: Album name.
            show_progress : Whether to display upload progress in the console.

        Returns:
            list[str]: Album media keys for all created albums.

        Raises:
            requests.HTTPError: If the API request fails.
            ValueError: If media_keys is empty.
        """
        album_limit = 20000  # Maximum number of items per album
        batch_size = 500  # Number of items to process per API call
        album_keys = []
        album_counter = 1

        if len(media_keys) > album_limit:
            self.logger.warning(
                f"{len(media_keys)} items exceed the album limit of {album_limit}. They will be split into multiple albums."
            )

        # Initialize progress bar
        progress = Progress(
            TextColumn("{task.description}"),
            SpinnerColumn(),
            MofNCompleteColumn(),
            TimeElapsedColumn(),
        )
        task = progress.add_task(
            f"[bold yellow]Adding items to album[/bold yellow] [cyan]{album_name}[/cyan]:",
            total=len(media_keys),
        )

        # context = (show_progress and Live(progress)) or nullcontext()

        # with context:
        for i in range(0, len(media_keys), album_limit):
            album_batch = media_keys[i : i + album_limit]
            # Add a suffix if media_keys will not fit into a single album
            current_album_name = (
                f"{album_name} {album_counter}"
                if len(media_keys) > album_limit
                else album_name
            )
            current_album_key = None
            for j in range(0, len(album_batch), batch_size):
                batch = album_batch[j : j + batch_size]
                if current_album_key is None:
                    # Create the album with the first batch
                    current_album_key = await self.api.create_album(
                        album_name=current_album_name, media_keys=batch
                    )
                    album_keys.append(current_album_key)
                else:
                    # Add to the existing album
                    await self.api.add_media_to_album(
                        album_media_key=current_album_key, media_keys=batch
                    )
                progress.update(task, advance=len(batch))
            album_counter += 1
        return album_keys

    async def update_cache(self, show_progress: bool = True):
        """
        Incrementally update local library cache.

        Args:
            show_progress: Whether to display progress in console.
        """
        # self.cache_dir.mkdir(parents=True, exist_ok=True)
        # progress = Progress(
        #     TextColumn("{task.description}"),
        #     SpinnerColumn(),
        #     "Updates: [green]{task.fields[updated]:>8}[/green]",
        #     "Deletions: [red]{task.fields[deleted]:>8}[/red]",
        # )
        columns = [
            TextColumn("{task.description}"),
            SpinnerColumn(),
            "Updates: [green]{task.fields[updated]:>8}[/green]",
            "Deletions: [red]{task.fields[deleted]:>8}[/red]",
        ]
        progress = self.live.get_progress("cache")
        if not progress:
            progress = await self.live.add_progress(
                progress_id="cache", columns=columns, title="Library Manager"
            )

        task_id = progress.add_task(
            "[bold magenta]Updating local cache[/bold magenta]:",
            updated=0,
            deleted=0,
        )

        # live.add(progress)
        # context = (show_progress and Live(progress)) or nullcontext()

        # with context:
        # Get saved state tokens
        async with AsyncStorage(self.dsn) as storage:
            init_state = await storage.get_init_state()

        if not init_state:
            self.logger.info("Cache Initiation")
            await self._cache_init(progress, task_id)
            async with AsyncStorage(self.dsn) as storage:
                await storage.set_init_state(1)

        self.logger.info("Cache Update")
        await self._cache_update(progress, task_id)
        # with Storage(self.db_path) as storage:
        #     init_state = storage.get_init_state()

        # if not init_state:
        #     self.logger.info("Cache Initiation")
        #     await self._cache_init(progress, task_id)
        #     with Storage(self.db_path) as storage:
        #         storage.set_init_state(1)
        # self.logger.info("Cache Update")
        # await self._cache_update(progress, task_id)

    async def _cache_update(self, progress, task_id):
        async with AsyncStorage(self.dsn) as storage:
            state_token, _ = await storage.get_state_tokens()

        response = await self.api.get_library_state(state_token)
        next_state_token, next_page_token, remote_media, media_keys_to_delete = (
            parse_db_update(response)
        )

        async with AsyncStorage(self.dsn) as storage:
            await storage.update_state_tokens(next_state_token, next_page_token)
            await storage.update(remote_media)
            await storage.delete(media_keys_to_delete)

        task = progress.tasks[int(task_id)]
        progress.update(
            task_id,
            updated=task.fields["updated"] + len(remote_media),
            deleted=task.fields["deleted"] + len(media_keys_to_delete),
        )

        if next_page_token:
            await self._process_pages(progress, task_id, state_token, next_page_token)

    async def _cache_init(self, progress, task_id):
        async with AsyncStorage(self.dsn) as storage:
            state_token, next_page_token = await storage.get_state_tokens()

        if next_page_token:
            await self._process_pages_init(progress, task_id, next_page_token)

        response = await self.api.get_library_state(state_token)
        state_token, next_page_token, remote_media, _ = parse_db_update(response)

        async with AsyncStorage(self.dsn) as storage:
            await storage.update_state_tokens(state_token, next_page_token)
            await storage.update(remote_media)

        task = progress.tasks[int(task_id)]
        progress.update(task_id, updated=task.fields["updated"] + len(remote_media))

        if next_page_token:
            await self._process_pages_init(progress, task_id, next_page_token)

    async def _process_pages_init(
        self, progress: Progress, task_id: TaskID, page_token: str
    ):
        """
        Process paginated results during cache update.

        Args:
            progress: Rich Progress object for tracking.
            task_id: ID of the progress task.
            page_token: Token for fetching page of results.
        """
        next_page_token: str | None = page_token
        while True:
            response = await self.api.get_library_page_init(next_page_token)
            _, next_page_token, remote_media, media_keys_to_delete = parse_db_update(
                response
            )

            async with AsyncStorage(self.dsn) as storage:
                await storage.update_state_tokens(page_token=next_page_token)
                await storage.update(remote_media)
                await storage.delete(media_keys_to_delete)

            task = progress.tasks[int(task_id)]
            progress.update(
                task_id,
                updated=task.fields["updated"] + len(remote_media),
                deleted=task.fields["deleted"] + len(media_keys_to_delete),
            )
            if not next_page_token:
                break

    async def _process_pages(
        self, progress: Progress, task_id: TaskID, state_token: str, page_token: str
    ):
        """
        Process paginated results during cache update.

        Args:
            progress: Rich Progress object for tracking.
            task_id: ID of the progress task.
            page_token: Token for fetching page of results.
        """
        next_page_token: str | None = page_token
        while True:
            response = await self.api.get_library_page(next_page_token, state_token)
            _, next_page_token, remote_media, media_keys_to_delete = parse_db_update(
                response
            )

            async with AsyncStorage(self.dsn) as storage:
                await storage.update_state_tokens(page_token=next_page_token)
                await storage.update(remote_media)
                await storage.delete(media_keys_to_delete)

            task = progress.tasks[int(task_id)]
            progress.update(
                task_id,
                updated=task.fields["updated"] + len(remote_media),
                deleted=task.fields["deleted"] + len(media_keys_to_delete),
            )
            if not next_page_token:
                break

    async def search_media(
        self,
        file_name: str | None = None,
        parsed_name: str | None = None,
        size_bytes: int | None = None,
        utc_timestamp: int | None = None,
    ) -> str | None:
        """
        Search for a media_key using metadata.
        Priority: size_bytes + utc_timestamp > file_name > parsed_name.
        """
        async with AsyncStorage(self.dsn) as db:
            return await db.search_media_key(
                file_name=file_name,
                parsed_name=parsed_name,
                size_bytes=size_bytes,
                utc_timestamp=utc_timestamp,
            )

    async def get_media_by_key(self, media_key: str) -> dict | None:
        """
        Retrieve full metadata for a given media_key.
        """
        async with AsyncStorage(self.dsn) as db:
            return await db.get_item_by_media_key(media_key)

    async def update_parsed_name(self, media_key: str, new_name: str) -> bool:
        """
        Update the parsed_name for a given media_key.
        Returns True if update occurred.
        """
        async with AsyncStorage(self.dsn) as db:
            return await db.update_parsed_name(media_key, new_name)

    async def get_download_url(self, media_key: str):
        try:
            response = await self.api.get_download_url(media_key)

            fname = response["1"]["2"]["4"]
            fsize = response["1"]["2"]["10"]
            url = response["1"]["5"]["3"]["5"]
            self.logger.info(f"Found {fname=} {fsize=} {url=}")
            return url
        except Exception:
            return
