import base64
import hashlib
from pathlib import Path

import aiofiles
from rich.progress import Progress, TaskID


async def calculate_sha1_hash(
    file_path: Path, progress: Progress, file_progress_id: TaskID
) -> tuple[bytes, str]:
    """
    Asynchronously calculate the SHA1 hash of a file in chunks, with progress tracking.

    Args:
        file_path: The path to the file to be hashed.
        progress: A Progress instance for tracking hash calculation.
        file_progress_id: A TaskID for progress tracking.

    Returns:
        tuple[bytes, str]: A tuple containing the SHA1 hash in bytes and base64 encoded string format.
    """

    progress.update(
        task_id=file_progress_id,
        description=f"Calculating Hash: {file_path.name}"[:60],
        total=file_path.stat().st_size,
    )

    hash_sha1 = hashlib.sha1()
    total_read = 0

    async with aiofiles.open(file_path, "rb") as file:
        while True:
            chunk = await file.read(1024 * 1024)  # 1MB
            if not chunk:
                break
            hash_sha1.update(chunk)
            total_read += len(chunk)
            progress.update(file_progress_id, advance=len(chunk))

    hash_bytes = hash_sha1.digest()
    hash_b64 = base64.b64encode(hash_bytes).decode("utf-8")

    return hash_bytes, hash_b64


# def calculate_sha1_hash(
#     file_path: Path, progress: Progress, file_progress_id: TaskID
# ) -> tuple[bytes, str]:
#     """
#     Calculate the SHA1 hash of a file in chunks, with progress tracking.

#     Args:
#         file_path: The path to the file to be hashed.
#         progress: A Progress instance for tracking hash calculation.
#         file_progress_id: A TaskID for progress tracking.

#     Returns:
#         tuple[bytes, str]: A tuple containing the SHA1 hash in bytes and base64 encoded string format.
#     """
#     progress.update(
#         task_id=file_progress_id, description=f"Calculating Hash: {file_path.name}"
#     )

#     hash_sha1 = hashlib.sha1()

#     with progress.open(file_path, "rb", task_id=file_progress_id) as file:
#         for chunk in iter(lambda: file.read(1024 * 1024), b""):
#             hash_sha1.update(chunk)

#     hash_bytes = hash_sha1.digest()
#     hash_b64 = base64.b64encode(hash_bytes).decode("utf-8")

#     return hash_bytes, hash_b64


def convert_sha1_hash(sha1_hash: str | bytes) -> tuple[bytes, str]:
    """
    Convert a SHA1 hash from string or bytes format to bytes and base64 encoded string.

    Args:
        sha1_hash: The SHA1 hash as a string or bytes.

    Returns:
        tuple[bytes, str]: A tuple containing the SHA1 hash in bytes and base64 encoded string format.

    Raises:
        ValueError: If the hash format is invalid.
    """
    if isinstance(sha1_hash, bytes):
        hash_bytes = sha1_hash
        hash_b64 = base64.b64encode(sha1_hash).decode("utf-8")
    elif isinstance(sha1_hash, str):
        if _is_hash_hexadecimal(sha1_hash):
            # Convert hex string to bytes
            hash_bytes = bytes.fromhex(sha1_hash)
            hash_b64 = base64.b64encode(hash_bytes).decode("utf-8")
        else:
            # Assume base64 encoded
            hash_bytes = base64.b64decode(sha1_hash)
            hash_b64 = sha1_hash
    else:
        raise ValueError("Invalid hash format. Expected str or bytes.")

    return hash_bytes, hash_b64


def _is_hash_hexadecimal(string: str) -> bool:
    """
    Check if the given string is a valid hexadecimal representation of a SHA-1 hash.

    Args:
        string: The string to check.

    Returns:
        bool: True if the string is a valid hexadecimal SHA-1 hash, False otherwise.
    """
    return len(string) == 40 and all(c in "0123456789abcdefABCDEF" for c in string)
