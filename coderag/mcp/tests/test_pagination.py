"""Tests for coderag_mcp.pagination."""

from collections.abc import Sequence

import pytest

from coderag_mcp.pagination import (
    DEFAULT_PER_PAGE,
    MAX_PER_PAGE,
    normalize_page_args,
    paginate,
)


class TestDefaultPerPage:
    """Default pagination parameters."""

    def test_default_is_100(self):
        items = list(range(250))
        page_items, meta = paginate(items)
        assert len(page_items) == DEFAULT_PER_PAGE
        assert meta["page"] == 1
        assert meta["per_page"] == DEFAULT_PER_PAGE

    def test_page_2_returns_next_100(self):
        items = list(range(250))
        page_items, meta = paginate(items, page=2)
        assert page_items == list(range(100, 200))
        assert meta["page"] == 2
        assert meta["total"] == 250
        assert meta["has_more"] is True
        assert meta["next_page"] == 3


class TestEmptyInput:
    """Empty list edge cases."""

    def test_empty_list(self):
        items: list[int] = []
        page_items, meta = paginate(items)
        assert page_items == []
        assert meta["total"] == 0
        assert meta["total_pages"] == 1
        assert meta["has_more"] is False
        assert meta["next_page"] is None


class TestPartialLastPage:
    """Partial last page metadata."""

    def test_partial_last_page(self):
        items = list(range(250))
        page_items, meta = paginate(items, page=3)
        assert page_items == list(range(200, 250))
        assert meta["page"] == 3
        assert meta["total"] == 250
        assert meta["total_pages"] == 3
        assert meta["has_more"] is False
        assert meta["next_page"] is None


class TestInvalidArgs:
    """ValueError for invalid page and per_page."""

    def test_invalid_page_zero(self):
        with pytest.raises(ValueError):
            paginate([], page=0)

    def test_invalid_page_negative(self):
        with pytest.raises(ValueError):
            paginate([], page=-1)

    def test_invalid_per_page_zero(self):
        with pytest.raises(ValueError):
            paginate([], per_page=0)

    def test_invalid_per_page_negative(self):
        with pytest.raises(ValueError):
            paginate([], per_page=-1)

    def test_invalid_per_page_above_max(self):
        with pytest.raises(ValueError):
            paginate([], per_page=MAX_PER_PAGE + 1)


class TestNoMutation:
    """Original items must not be mutated."""

    def test_items_not_mutated(self):
        items: list[int] = list(range(1000))
        original = items.copy()
        paginate(items, page=2, per_page=100)
        assert items == original


class TestNormalizePageArgs:
    """normalize_page_args validation."""

    def test_valid_args_return(self):
        page, per_page = normalize_page_args(1, 50)
        assert page == 1
        assert per_page == 50

    def test_invalid_page_raises(self):
        with pytest.raises(ValueError):
            normalize_page_args(0, 50)
