from dataclasses import dataclass


@dataclass(slots=True)
class MediaItem:
    media_key: str
    file_name: str
    dedup_key: str
    is_canonical: bool
    type: int
    caption: str | None
    collection_id: str
    size_bytes: int
    quota_charged_bytes: int
    origin: str
    content_version: int
    utc_timestamp: int
    server_creation_timestamp: int
    timezone_offset: int | None = None
    width: int | None = None
    height: int | None = None
    remote_url: str = ""
    upload_status: int | None = None
    trash_timestamp: int | None = None
    is_archived: bool = False
    is_favorite: bool = False
    is_locked: bool = False
    is_original_quality: bool = False
    latitude: float | None = None
    longitude: float | None = None
    location_name: str | None = None
    location_id: str | None = None
    is_edited: bool = False
    make: str | None = None
    model: str | None = None
    aperture: float | None = None
    shutter_speed: float | None = None
    iso: int | None = None
    focal_length: float | None = None
    duration: int | None = None
    capture_frame_rate: float | None = None
    encoded_frame_rate: float | None = None
    is_micro_video: bool = False
    micro_video_width: int | None = None
    micro_video_height: int | None = None
    user_name: str | None = None
    name: str | None = None
    path: str | None = None
    


# @dataclass(slots=True)
# class CollectionItem:
#     collection_media_key: str
#     collection_album_id: str
#     title: str
#     total_items: int
#     type: int
#     sort_order: int
#     is_custom_ordered: bool
#     cover_item_media_key: str | None = None
#     start: int | None = None
#     end: int | None = None
#     last_activity_time_ms: int | None = None


# @dataclass(slots=True)
# class EnvelopeItem:
#     media_key: str
#     hint_time_ms: int
