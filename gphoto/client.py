import asyncio
import os
from pathlib import Path
from typing import Sequence

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
from utils.logger2 import logger, shutdown_event, sleep

from .api import DEFAULT_TIMEOUT, Api
from .db import AsyncStorage
from .db_update_parser import parse_db_update
from .hash_handler import calculate_sha1_hash, convert_sha1_hash


class GPhoto:
    """Google Photos client based on reverse engineered mobile API."""

    def __init__(
        self,
        path: str = "",
        user: str = "",
        filter_path: str = "",
        proxy: str = "",
        timeout: int = DEFAULT_TIMEOUT,
        threads: int = 1,
    ) -> None:
        self.path = path
        self.user = user
        self.timeout = timeout
        self.language = "en_US"
        self.threads = threads
        self.filter_path = filter_path
        self.api = Api(
            proxy=proxy, language=self.language, timeout=timeout, user=self.user
        )

        self.progress_update_cache = None
        self.columns_update_cache = [
            TextColumn("{task.description}"),
            SpinnerColumn(),
            "Updates: [green]{task.fields[updated]:>8}[/green]",
            "Deletions: [red]{task.fields[deleted]:>8}[/red]",
        ]

        self.progress_upload_monitor = None
        self.columns_upload_monitor = [
            TextColumn("[bold yellow]Uploader:"),
            SpinnerColumn(),
            MofNCompleteColumn(),
            TextColumn("{task.description}"),
        ]
        self.progress_upload_file = None
        self.columns_upload_file = [
            DownloadColumn(),
            TaskProgressColumn(),
            TimeRemainingColumn(),
            TransferSpeedColumn(),
            TextColumn("{task.description}"),
        ]
        self.progress_selection_monitor = None
        self.columns_selection_monitor = [
            TextColumn("[bold yellow]File Selection:"),
            SpinnerColumn(),
            MofNCompleteColumn(),
            TextColumn("{task.description}"),
        ]

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
        progress: Progress,
        threads: asyncio.Semaphore,
    ) -> dict[str, str]:
        async with threads:
            try:
                task_id = progress.add_task(description="")
                if shutdown_event.is_set():
                    raise asyncio.CancelledError()

                file_path = Path(file_path)
                file_size = file_path.stat().st_size

                # if

                hash_bytes, hash_b64 = await calculate_sha1_hash(
                    file_path, progress, task_id
                )

                if shutdown_event.is_set():
                    raise asyncio.CancelledError()

                progress.update(
                    task_id=task_id,
                    description=f"Checking: {file_path.name}"[:60],
                )
                if remote_media_key := await self.api.find_remote_media_by_hash(
                    hash_bytes
                ):  
                    # os.remove(file_path)               
                    logger.info(f"File already exists in Google Photos: {file_path}")
                    new = file_path.absolute().as_posix().replace("/gphotos/", "/scan/")
                    logger.info(f"{file_path} moving to scan directory: {new}")
                    os.renames(file_path, new)
                    return file_path.absolute().as_posix(), remote_media_key

                upload_token = await self.api.get_upload_token(hash_b64, file_size)
                progress.reset(task_id=task_id)
                progress.update(
                    task_id=task_id,
                    description=f"Uploading: {file_path.name}"[:60],
                )

                upload_response = await self.api.upload_file(
                    file=self._file_stream(file_path, progress, task_id),
                    upload_token=upload_token,
                )

                progress.update(
                    task_id=task_id,
                    description=f"Commit Upload: {file_path.name}"[:60],
                )
                last_modified_timestamp = int(os.path.getmtime(file_path))
                model = "Pixel XL"
                quality = "original"
                # if saver:
                #     quality = "saver"
                #     model = "Pixel 2"
                # if use_quota:
                #     model = "Pixel 8"
                media_key = await self.api.commit_upload(
                    upload_response_decoded=upload_response,
                    file_name=file_path.name,
                    sha1_hash=hash_bytes,
                    upload_timestamp=last_modified_timestamp,
                    model=model,
                    quality=quality,
                )
                return file_path.absolute().as_posix(), media_key
            except Exception as e:
                logger.exception(f"Error uploading file {file_path}: {e}")
                os.renames(file_path, file_path.absolute().as_posix().replace("/gphotos/", "/gphotos.error/"))                                

            finally:
                progress.remove_task(task_id)
                # progress.update(file_progress_id, visible=False)

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

    async def upload_monitor(self, delete_from_host: bool = True) -> dict[str, str]:
        await self.update_cache()
        logger.info(f"{self.user} gphotos upload monitor started...")
        progress = self.progress_upload_monitor
        task_id = progress.add_task(f"{self.user}'s file:")
        while not shutdown_event.is_set():
            results = {}
            upload_error_count = 0

            files_to_be_upload = set()
            for root, _, files in os.walk(self.path):
                if shutdown_event.is_set():
                    break

                if self.filter_path not in root:
                    continue

                for file in files:
                    if file.endswith(".empty"):
                        continue

                    filepath = os.path.join(root, file)

                    if os.path.getsize(filepath) > 100 * 1024**3:
                        os.renames(
                            filepath, filepath.replace("/gphotos/", "/gphotos.large/")
                        )
                        logger.warning(f"File is larger than 100GB: {file=}")
                    else:
                        files_to_be_upload.add(filepath)

            if files_to_be_upload:
                progress.reset(
                    task_id=task_id, total=len(files_to_be_upload), visible=True
                )

                upload_threads = asyncio.Semaphore(self.threads)

                tasks = [
                    asyncio.create_task(
                        self._upload_file(
                            file,
                            progress=self.progress_upload_file,
                            threads=upload_threads,
                        )
                    )
                    for file in files_to_be_upload
                ]

                for coro in asyncio.as_completed(tasks):
                    try:
                        file, media_key = await coro
                        logger.info({file: media_key})
                        results = results | {file: media_key}
                        # if delete_from_host:
                            
                            # os.remove(file)

                    except HTTPStatusError as e:
                        logger.error(f"HTTP error occurred {file}: {e}")
                        if e.response.status_code == 401:
                            self.api.token = await self.api.get_auth_token()

                        upload_error_count += 1
                        progress.update(
                            task_id=task_id,
                            description=f"[bold red] Errors: {upload_error_count}",
                        )
                    except asyncio.CancelledError:
                        pass
                    except Exception as e:
                        logger.exception(f"Upload error occurred {e}")

                        upload_error_count += 1
                        progress.update(
                            task_id=task_id,
                            description=f"[bold red] Errors: {upload_error_count}",
                        )
                    finally:
                        progress.advance(task_id)

                if results:
                    await self.update_cache()
            else:
                progress.reset(task_id=task_id, visible=False)

                # if album_name:
                #     await self._handle_album_creation(results, album_name, show_progress)

            await sleep(30)
        progress.remove_task(task_id=task_id)
        logger.info(f"{self.user} gphotos upload monitor stop")

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

    async def move_to_trash(self, dedup_keys: Sequence[str | bytes]) -> dict:
        """
        Move remote media files to trash.

        Args:
            sha1_hashes: Single SHA-1 hash or sequence of hashes to move to trash.

        Returns:
            dict: API response containing operation results.

        Raises:
            ValueError: If input hashes are invalid.
        """

        # if isinstance(sha1_hashes, (str, bytes)):
        #     sha1_hashes = [sha1_hashes]

        # try:
        #     # Convert all hashes to Base64 format
        #     hashes_b64 = [convert_sha1_hash(hash)[1] for hash in sha1_hashes]  # type: ignore
        #     dedup_keys = [utils.urlsafe_base64(hash) for hash in hashes_b64]
        # except (TypeError, ValueError) as e:
        #     raise ValueError("Invalid SHA-1 hash format") from e

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
            logger.warning(
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

    async def update_cache(self):
        logger.info(f"Updating {self.user}'s cache")
        progress = self.progress_update_cache
        if not progress.task_ids:
            task_id = progress.add_task(
                f"{self.user}'s cache",
                updated=0,
                deleted=0,
            )
        else:
            task_id = progress.task_ids[0]
            progress.update(task_id=task_id, visible=True)

        async with AsyncStorage(self.user) as storage:
            init_state = await storage.get_init_state()

        if not init_state:
            logger.info("Cache Initiation")
            await self._cache_init(progress, task_id)
            async with AsyncStorage(self.user) as storage:
                await storage.set_init_state(1)

        await self._cache_update(progress, task_id)
        task = progress.tasks[int(task_id)]
        updated = task.fields["updated"]
        deleted = task.fields["deleted"]

        progress.update(task_id=task_id, visible=False)
        logger.info(f"{self.user}'s cache updated. {updated=} {deleted=}")

    async def _cache_update(self, progress, task_id):
        async with AsyncStorage(self.user) as storage:
            state_token, _ = await storage.get_state_tokens()

        response = await self.api.get_library_state(state_token)
        try:
            next_state_token, next_page_token, remote_media, media_keys_to_delete = (
                parse_db_update(response)
            )

            async with AsyncStorage(self.user) as storage:
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
                await self._process_pages(
                    progress, task_id, state_token, next_page_token
                )
        except Exception:
            pass

    async def _cache_init(self, progress, task_id):
        async with AsyncStorage(self.user) as storage:
            state_token, next_page_token = await storage.get_state_tokens()

        if next_page_token:
            await self._process_pages_init(progress, task_id, next_page_token)

        response = await self.api.get_library_state(state_token)
        state_token, next_page_token, remote_media, _ = parse_db_update(response)

        async with AsyncStorage(self.user) as storage:
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
        while not shutdown_event.is_set():
            response = await self.api.get_library_page_init(next_page_token)
            _, next_page_token, remote_media, media_keys_to_delete = parse_db_update(
                response
            )

            async with AsyncStorage(self.user) as storage:
                await storage.update_state_tokens(page_token=next_page_token)
                await storage.update(remote_media)
                await storage.delete(media_keys_to_delete)
            try:
                task = progress.tasks[int(task_id)]
                progress.update(
                    task_id,
                    updated=task.fields["updated"] + len(remote_media),
                    deleted=task.fields["deleted"] + len(media_keys_to_delete),
                )
            except Exception:
                pass

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
        while not shutdown_event.is_set():
            response = await self.api.get_library_page(next_page_token, state_token)
            _, next_page_token, remote_media, media_keys_to_delete = parse_db_update(
                response
            )

            async with AsyncStorage(self.user) as storage:
                await storage.update_state_tokens(page_token=next_page_token)
                await storage.update(remote_media)
                await storage.delete(media_keys_to_delete)
            try:
                task = progress.tasks[int(task_id)]
                progress.update(
                    task_id,
                    updated=task.fields["updated"] + len(remote_media),
                    deleted=task.fields["deleted"] + len(media_keys_to_delete),
                )
            except Exception:
                pass

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
        Priority: size_bytes + utc_timestamp > size_bytes > file_name > parsed_name.
        """
        async with AsyncStorage(self.user) as db:
            return await db.search(
                file_name=file_name,
                parsed_name=parsed_name,
                size_bytes=size_bytes,
                utc_timestamp=utc_timestamp,
            )

    async def get_media_by_key(self, media_key: str) -> dict | None:
        """
        Retrieve full metadata for a given media_key.
        """
        async with AsyncStorage(self.user) as db:
            return await db.get_item_by_media_key(media_key)

    async def update_parsed_name(self, media_key: str, new_name: str) -> bool:
        """
        Update the parsed_name for a given media_key.
        Returns True if update occurred.
        """
        async with AsyncStorage(self.user) as db:
            return await db.update_parsed_name(media_key, new_name)

    async def get_download_url(self, media_key: str):
        try:
            item = await self.get_media_by_key(media_key)
            response = await self.api.get_download_url(media_key, item.get("user_name"))
            fname = response["1"]["2"]["4"]
            fsize = response["1"]["2"]["10"]
            url = response["1"]["5"]["3"]["5"]
            logger.info(f"Found {fname=} {fsize=} {url=}")
            return url
        except Exception:
            return

    async def get_stream_url(self, media_key: str):
        try:
            if media_key.startswith("AF1QipM-"):
                item = await self.get_media_by_key(media_key)
            else:
                item = await self.search_media(file_name=media_key)
                media_key = item.get("media_key")

            if media_key:
                logger.info(
                    f"Getting data from gphots {item.get('file_name')=} {item.get('media_key')=} {item.get('user_name')=}"
                )
                response = await self.api.get_stream_url(
                    media_key,
                    version=item.get("content_version"),
                    user=item.get("user_name"),
                )
                return response
        except Exception:
            return

    async def get_stream_url_from_file(self, media_key: str):
        try:
            item = await self.search_media(file_name=media_key)
            response = await self.api.get_stream_url(
                item.get("media_key"),
                version=item.get("content_version"),
                user=item.get("user_name"),
            )
            return response
        except Exception:
            return

    async def get_duplicates(self):
        async with AsyncStorage(self.user) as db:
            duplicates = await db.get_duplicate_files_by_name_and_size()
            return duplicates

    async def _set_up_progress_bars(self, live: DynamicLive):
        self.progress_update_cache = await live.add_progress(
            f"cache_{self.user}", self.columns_update_cache
        )

        self.progress_upload_monitor = await live.add_progress(
            f"download_monitor_{self.user}", self.columns_upload_monitor
        )

        self.progress_upload_file = await live.add_progress(
            f"download_{self.user}", self.columns_upload_file
        )

    async def monitor(
        self, live_progress: DynamicLive, update_cache_only: bool = False
    ):
        await self._set_up_progress_bars(live_progress)

        if not update_cache_only:
            await self.upload_monitor()
            return

        while not shutdown_event.is_set():
            await self.update_cache()
            await sleep(60 * 5)
