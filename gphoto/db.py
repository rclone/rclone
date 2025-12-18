from dataclasses import asdict
from typing import Iterable, Self, Sequence
from urllib.parse import urlparse, urlunparse

import asyncpg
import psycopg  # sync used for DB creation

from .models import MediaItem


class AsyncStorage:
    def __init__(self, user: str = '') -> None:
        self.dsn = "postgresql://vavtnen:637578@localhost:10012/gphotos"
        self.conn: asyncpg.Connection | None = None
        self.user = user

    async def __aenter__(self) -> Self:
        await self._ensure_database()
        self.conn = await asyncpg.connect(self.dsn)
        await self._create_tables()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        await self.conn.close()

    async def _ensure_database(self) -> None:
        """
        Ensure the target database exists. Uses psycopg (sync) to create if missing.
        """
        parsed = urlparse(self.dsn)
        db_name = parsed.path.lstrip("/")  # 'gphotos'

        # Connect to 'postgres' DB first
        admin_dsn = urlunparse(parsed._replace(path="/postgres"))

        with psycopg.connect(admin_dsn, autocommit=True) as conn:
            with conn.cursor() as cur:
                cur.execute("SELECT 1 FROM pg_database WHERE datname = %s", (db_name,))
                if not cur.fetchone():
                    cur.execute(f'CREATE DATABASE "{db_name}"')
                    print(f"✅ Created database '{db_name}'")

    async def _create_tables(self) -> None:
        await self.conn.execute("""
            CREATE TABLE IF NOT EXISTS remote_media (
                media_key TEXT PRIMARY KEY,
                file_name TEXT,
                dedup_key TEXT,
                is_canonical BOOLEAN,
                type INTEGER,
                caption TEXT,
                collection_id TEXT,
                size_bytes BIGINT,
                quota_charged_bytes BIGINT,
                origin TEXT,
                content_version INTEGER,
                utc_timestamp BIGINT,
                server_creation_timestamp BIGINT,
                timezone_offset INTEGER,
                width INTEGER,
                height INTEGER,
                remote_url TEXT,
                upload_status INTEGER,
                trash_timestamp BIGINT,
                is_archived BOOLEAN,
                is_favorite BOOLEAN,
                is_locked BOOLEAN,
                is_original_quality BOOLEAN,
                latitude DOUBLE PRECISION,
                longitude DOUBLE PRECISION,
                location_name TEXT,
                location_id TEXT,
                is_edited BOOLEAN,
                make TEXT,
                model TEXT,
                aperture DOUBLE PRECISION,
                shutter_speed DOUBLE PRECISION,
                iso INTEGER,
                focal_length DOUBLE PRECISION,
                duration INTEGER,
                capture_frame_rate DOUBLE PRECISION,
                encoded_frame_rate DOUBLE PRECISION,
                is_micro_video BOOLEAN,
                micro_video_width INTEGER,
                micro_video_height INTEGER,
                user_name TEXT,
                name TEXT,
                path TEXT                
            )
        """)

        await self.conn.execute("""
            CREATE TABLE IF NOT EXISTS state (
                id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
                state_token TEXT,
                page_token TEXT,
                init_complete BOOLEAN,
                user_name TEXT UNIQUE
            )
        """)

    async def update(self, items: Iterable[MediaItem]) -> None:
        if not items:
            return

        items_dicts = [{**asdict(item), "user_name": self.user} for item in items]
        columns = items_dicts[0].keys()
        placeholders = ", ".join(f"${i + 1}" for i in range(len(columns)))
        updates = ", ".join(
            f"{col} = EXCLUDED.{col}" for col in columns if col != "media_key"
        )
        columns_str = ", ".join(columns)

        sql = f"""
            INSERT INTO remote_media ({columns_str})
            VALUES ({placeholders})
            ON CONFLICT (media_key) DO UPDATE SET {updates}
        """

        for item in items_dicts:
            values = list(item.values())
            # values.append(self.user)
            await self.conn.execute(sql, *values)

    async def delete(self, media_keys: Sequence[str]) -> None:
        if not media_keys:
            return

        await self.conn.execute(
            "DELETE FROM remote_media WHERE media_key = ANY($1)", media_keys
        )

    async def get_state_tokens(self) -> tuple[str, str]:
        row = await self.conn.fetchrow(
            "SELECT state_token, page_token FROM state WHERE user_name = $1", self.user
        )
        return (row["state_token"], row["page_token"]) if row else ("", "")

    async def update_state_tokens(
        self, state_token: str | None = None, page_token: str | None = None
    ) -> None:
        updates = []
        params = []
        param_index = 1  # PostgreSQL uses 1-based parameter indexing

        if state_token is not None:
            updates.append(f"state_token = ${param_index}")
            params.append(state_token)
            param_index += 1

        if page_token is not None:
            updates.append(f"page_token = ${param_index}")
            params.append(page_token)
            param_index += 1

        if updates:
            updates_str = ", ".join(updates)
            sql = f"UPDATE state SET {updates_str} WHERE user_name = ${param_index}"
            params.append(self.user)
            await self.conn.execute(sql, *params)

    async def get_init_state(self) -> bool:
        row = await self.conn.fetchrow(
            "SELECT init_complete FROM state WHERE user_name = $1", self.user
        )
        if not row:
            await self.conn.execute(
                """
            INSERT INTO state (state_token, page_token, init_complete, user_name)
            VALUES ('', '', FALSE, $1)
            ON CONFLICT (user_name) DO NOTHING
            """,
                self.user,
            )
        return bool(row["init_complete"]) if row else False

    async def set_init_state(self, state: int) -> None:
        await self.conn.execute(
            "UPDATE state SET init_complete = $1 WHERE user_name = $2",
            bool(state),
            self.user,
        )

    async def search(
        self,
        file_name: str | None = None,
        parsed_name: str | None = None,
        size_bytes: int | None = None,
        utc_timestamp: int | None = None,
    ) -> str | None:
        if size_bytes and utc_timestamp:
            row = await self.conn.fetchrow(
                """
                SELECT * FROM remote_media
                WHERE size_bytes = $1
                AND (utc_timestamp / 1000)::BIGINT BETWEEN $2 - 1 AND $2 + 1
                LIMIT 1
            """,
                size_bytes,
                utc_timestamp,
            )

            if row:
                return row

        if size_bytes:
            row = await self.conn.fetchrow(
                """
                SELECT * FROM remote_media
                WHERE size_bytes = $1                
                LIMIT 1
            """,
                size_bytes,
            )
            if row:
                return row

        if file_name:
            row = await self.conn.fetchrow(
                """
                SELECT * FROM remote_media
                WHERE file_name = $1
                LIMIT 1
            """,
                file_name,
            )
            if row:
                return row

        if parsed_name:
            row = await self.conn.fetchrow(
                """
                SELECT * FROM remote_media
                WHERE parsed_name = $1
                LIMIT 1
            """,
                parsed_name,
            )
            if row:
                return row

        return None

    async def get_item_by_media_key(self, media_key: str) -> dict | None:
        row = await self.conn.fetchrow(
            "SELECT * FROM remote_media WHERE media_key = $1", media_key
        )
        return dict(row) if row else None

    async def update_parsed_name(self, media_key: str, new_parsed_name: str) -> bool:
        result = await self.conn.execute(
            """
            UPDATE remote_media
            SET parsed_name = $1
            WHERE media_key = $2
        """,
            new_parsed_name,
            media_key,
        )
        return result.endswith("UPDATE 1")

    async def get_duplicate_files_by_name_and_size(self) -> list[dict]:
        """
        Return all media entries (media_key, file_name, size_bytes)
        that have duplicates based on file_name and size_bytes.
        """
        # rows = await self.conn.fetch("""
        #     SELECT media_key, file_name, size_bytes
        #     FROM remote_media
        #     WHERE (file_name, size_bytes) IN (
        #         SELECT file_name, size_bytes
        #         FROM remote_media
        #         GROUP BY file_name, size_bytes
        #         HAVING COUNT(*) > 1
        #     )
        #     ORDER BY file_name, size_bytes, media_key
        # """)
        rows = await self.conn.fetch("""
            WITH duplicates AS (
                SELECT
                    dedup_key,
                    file_name,
                    size_bytes,
                    ROW_NUMBER() OVER (
                        PARTITION BY file_name, size_bytes ORDER BY media_key
                    ) AS rk
                FROM remote_media
                WHERE dedup_key IS NOT NULL
            )
            SELECT dedup_key
            FROM duplicates
            WHERE rk > 1
            ORDER BY file_name, size_bytes
        """)

        return [str(row["dedup_key"]) for row in rows]

    async def close(self) -> None:
        await self.conn.close()
