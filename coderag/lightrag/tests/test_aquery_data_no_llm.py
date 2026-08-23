# Unit tests: /query/data must be LLM-free vector search.
#
# aquery_data() presets the raw query as hl/ll keywords so
# get_keywords_from_query() short-circuits instead of calling the LLM for
# keyword extraction; rerank is force-disabled on this path. These tests guard
# that behavior against regressions.

import asyncio
import inspect

from lightrag.base import QueryParam
from lightrag.lightrag import LightRAG
from lightrag.operate import get_keywords_from_query


def test_preset_keywords_skip_llm_extraction(monkeypatch):
    """With keywords pre-set on QueryParam the LLM keyword extractor is never called."""

    async def _must_not_be_called(*args, **kwargs):  # fails the test if invoked
        raise AssertionError(
            "extract_keywords_only must not run when keywords are preset"
        )

    monkeypatch.setattr("lightrag.operate.extract_keywords_only", _must_not_be_called)

    param = QueryParam(mode="hybrid", hl_keywords=["Foo"], ll_keywords=["foo bar"])

    async def run():
        return await get_keywords_from_query("some query", param, {}, None)

    hl, ll = asyncio.run(run())
    assert hl == ["Foo"]
    assert ll == ["foo bar"]


def test_aquery_data_presets_query_as_keywords_and_disables_rerank():
    """Regression guard: aquery_data embeds the raw query instead of LLM keywords."""
    src = inspect.getsource(LightRAG.aquery_data)
    assert "hl_keywords=param.hl_keywords or [query.strip()]" in src
    assert "ll_keywords=param.ll_keywords or [query.strip()]" in src
    assert "enable_rerank=False" in src
