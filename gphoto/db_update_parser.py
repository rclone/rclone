import base64

from .models import MediaItem
from .utils import int64_to_float, int32_to_float, fixed32_to_float, urlsafe_base64


def _parse_media_item(d: dict) -> MediaItem:
    """Parse a single media item from the raw data."""

    dedup_key = next((d["2"]["21"][key] for key in d["2"]["21"] if key.startswith("1")), "")
    if not isinstance(dedup_key, str):
        try:
            dedup_key = urlsafe_base64(base64.b64encode(d["2"]["13"]["1"]).decode())
        except Exception as e:
            raise RuntimeError("Error parsing dedup_key") from e

    origin_map = {
        1: "self",
        3: "partner",
        4: "shared",
    }

    item = MediaItem(
        media_key=d["1"],
        caption=next((d["2"][key] for key in d["2"] if key.startswith("3")), "") or None,
        file_name=d["2"]["4"],
        dedup_key=dedup_key,
        is_canonical=not any(prop.get("1") == 27 for prop in d["2"]["5"]),
        type=d["5"]["1"],
        collection_id=d["2"]["1"]["1"],
        size_bytes=d["2"]["10"],
        timezone_offset=d["2"].get("8", 0),
        utc_timestamp=d["2"]["7"],
        server_creation_timestamp=d["2"]["9"],
        upload_status=d["2"]["11"],
        quota_charged_bytes=d["2"]["35"]["2"],
        origin=origin_map[d["2"]["30"]["1"]],
        content_version=d["2"]["26"],
        trash_timestamp=d["2"]["16"].get("3", 0),
        is_archived=d["2"]["29"]["1"] == 1,
        is_favorite=d["2"]["31"]["1"] == 1,
        is_locked=d["2"]["39"]["1"] == 1,
        is_original_quality=d["2"]["35"]["3"] == 2,
    )

    if d["17"].get("1"):
        item.latitude = fixed32_to_float(d["17"]["1"]["1"])
        item.longitude = fixed32_to_float(d["17"]["1"]["2"])
    if d["17"].get("5"):
        item.location_name = d["17"]["5"]["2"]["1"]
        item.location_id = d["17"]["5"]["3"]

    if d["5"].get("2"):
        # photo
        item.is_edited = "4" in d["5"]["2"]
        item.remote_url = d["5"]["2"]["1"]["1"]
        item.width = d["5"]["2"]["1"]["9"]["1"]
        item.height = d["5"]["2"]["1"]["9"]["2"]
        if d["5"]["2"]["1"]["9"].get("5"):
            item.make = d["5"]["2"]["1"]["9"]["5"].get("1")
            item.model = d["5"]["2"]["1"]["9"]["5"].get("2")
            item.aperture = d["5"]["2"]["1"]["9"]["5"].get("4") and int32_to_float(d["5"]["2"]["1"]["9"]["5"]["4"])
            item.shutter_speed = d["5"]["2"]["1"]["9"]["5"].get("5") and int32_to_float(d["5"]["2"]["1"]["9"]["5"]["5"])
            item.iso = d["5"]["2"]["1"]["9"]["5"].get("6")
            item.focal_length = d["5"]["2"]["1"]["9"]["5"].get("7") and int32_to_float(d["5"]["2"]["1"]["9"]["5"]["7"])

    if d["5"].get("3"):
        # video
        item.remote_url = d["5"]["3"]["2"]["1"]
        if d["5"]["3"].get("4"):
            item.duration = d["5"]["3"]["4"].get("1")
            item.width = d["5"]["3"]["4"].get("4")
            item.height = d["5"]["3"]["4"].get("5")
        item.capture_frame_rate = d["5"]["3"].get("6", {}).get("4") and int64_to_float(d["5"]["3"]["6"]["4"])
        item.encoded_frame_rate = d["5"]["3"].get("6", {}).get("5") and int64_to_float(d["5"]["3"]["6"]["5"])

    if d["5"].get("5", {}).get("2", {}).get("4"):
        # micro video
        item.is_micro_video = True
        item.duration = d["5"]["5"]["2"]["4"]["1"]
        item.micro_video_width = d["5"]["5"]["2"]["4"]["4"]
        item.micro_video_height = d["5"]["5"]["2"]["4"]["5"]

    return item


def _parse_deletion_item(d: dict) -> str | None:
    """Parse a single deletion item from the raw data."""
    type = d["1"]["1"]
    if type == 1:
        return d["1"]["2"]["1"]
    return None
    # if type == 4:
    #     return d["1"]["5"]["2"]
    # if type == 6:
    #     return d["1"]["7"]["1"]


# def _parse_collection_item(d: dict) -> CollectionItem:
#     """Parse a single collection item from the raw data."""
#     return CollectionItem(
#         collection_media_key=d["1"],
#         collection_album_id=d["4"]["2"]["3"],
#         cover_item_media_key=d["2"].get("17", {}).get("1"),
#         start=d["2"]["10"]["6"]["1"],
#         end=d["2"]["10"]["7"]["1"],
#         last_activity_time_ms=d["2"]["10"]["10"],
#         title=d["2"]["5"],
#         total_items=d["2"]["7"],
#         type=d["2"]["8"],
#         sort_order=d["19"]["1"],
#         is_custom_ordered=d["19"]["2"] == 1,
#     )


# def _parse_envelope_item(d: dict) -> EnvelopeItem:
#     """Parse a single envelope item from the raw data."""
#     return EnvelopeItem(media_key=d["1"]["1"], hint_time_ms=d["2"])


def _get_items_list(data: dict, key: str) -> list[dict]:
    """Helper to get a list of items from the data, handling single item case."""
    items = data["1"].get(key, [])
    return [items] if isinstance(items, dict) else items


def parse_db_update(data: dict) -> tuple[str, str | None, list[MediaItem], list[str]]:
    """Parse the library state from the raw data."""
    next_page_token = data["1"].get("1", "")
    state_token = data["1"].get("6", "")

    # Parse media item
    remote_media = []
    media_items = _get_items_list(data, "2")
    remote_media.extend(_parse_media_item(d) for d in media_items)

    media_keys_to_delete = []
    deletions = _get_items_list(data, "9")
    for d in deletions:
        if media_key := _parse_deletion_item(d):
            media_keys_to_delete.append(media_key)

    # collections = _get_items_list(data, "3")
    # remote_media.extend(_parse_collection_item(d) for d in collections)

    # envelopes = _get_items_list(data, "12")
    # for d in envelopes:
    #     _parse_envelope_item(d)

    return state_token, next_page_token, remote_media, media_keys_to_delete
