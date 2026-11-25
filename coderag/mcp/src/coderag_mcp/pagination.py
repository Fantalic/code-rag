"""Pure, reusable pagination helper independent of LightRAG, FastMCP, and Pydantic."""

from collections.abc import Sequence
from math import ceil
from typing import Any, TypeVar

T = TypeVar("T")

DEFAULT_PER_PAGE = 100
MAX_PER_PAGE = 1000


def normalize_page_args(page: int, per_page: int) -> tuple[int, int]:
    """Validate and return (page, per_page).

    Raises ValueError if page < 1 or per_page is out of range.
    """
    if page < 1:
        raise ValueError(f"page must be >= 1, got {page}")
    if not (1 <= per_page <= MAX_PER_PAGE):
        raise ValueError(
            f"per_page must be between 1 and {MAX_PER_PAGE}, got {per_page}"
        )
    return (page, per_page)


def paginate(
    items: Sequence[T],
    page: int = 1,
    per_page: int = DEFAULT_PER_PAGE,
) -> tuple[list[T], dict[str, Any]]:
    """Slice *items* into a single page and return metadata.

    Returns a ``(page_items, metadata)`` tuple where *metadata* contains
    ``page``, ``per_page``, ``total``, ``total_pages``, ``has_more``, and
    ``next_page``.
    """
    page, per_page = normalize_page_args(page, per_page)
    total = len(items)
    start = (page - 1) * per_page
    end = start + per_page
    page_items = list(items[start:end])
    total_pages = max(1, ceil(total / per_page))
    has_more = page < total_pages
    metadata: dict[str, Any] = {
        "page": page,
        "per_page": per_page,
        "total": total,
        "total_pages": total_pages,
        "has_more": has_more,
        "next_page": page + 1 if has_more else None,
    }
    return (page_items, metadata)
