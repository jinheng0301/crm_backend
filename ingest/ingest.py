"""Download, parse, chunk, embed, and upsert the official tax sources."""

from __future__ import annotations

import hashlib
import os
import re
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable
from urllib.parse import urlparse

import fitz
import psycopg
import requests
import yaml
from bs4 import BeautifulSoup
from dotenv import load_dotenv

ROOT = Path(__file__).resolve().parents[1]
DATA_DIR = Path(__file__).resolve().parent / "data"
load_dotenv(ROOT / ".env")
CHUNK_CHARS = int(os.getenv("RAG_CHUNK_CHARS", "2800"))
OVERLAP_CHARS = int(os.getenv("RAG_OVERLAP_CHARS", "350"))
EMBED_BATCH_SIZE = 16
HTTP = requests.Session()
HTTP.headers.update({"User-Agent": "taxPOD-CRM-RAG/1.0 (document ingestion)"})


@dataclass
class TextPage:
    number: int
    text: str


@dataclass
class Chunk:
    index: int
    page_start: int
    page_end: int
    section: str
    content: str
    token_count: int


def load_sources() -> list[dict[str, str]]:
    with (Path(__file__).parent / "sources.yaml").open("r", encoding="utf-8") as source_file:
        config = yaml.safe_load(source_file)
    docs = config.get("documents", [])
    if len(docs) < 5:
        raise ValueError("sources.yaml must contain at least five public sources")
    return docs


def download(url: str) -> tuple[bytes, str]:
    response = HTTP.get(url, timeout=(10, 60))
    response.raise_for_status()
    body = response.content
    content_type = response.headers.get("Content-Type", "").lower()
    if not body:
        raise ValueError(f"empty response from {url}")
    return body, content_type


def parse_document(body: bytes, content_type: str, url: str) -> list[TextPage]:
    if body.startswith(b"%PDF-") or "application/pdf" in content_type or url.lower().endswith(".pdf"):
        with fitz.open(stream=body, filetype="pdf") as pdf:
            pages = [TextPage(i + 1, page.get_text("text")) for i, page in enumerate(pdf)]
        return [page for page in pages if page.text.strip()]

    soup = BeautifulSoup(body, "html.parser")
    for tag in soup(["script", "style", "nav", "footer", "header", "noscript", "svg"]):
        tag.decompose()
    main = soup.find("main") or soup.find("article") or soup.body or soup
    text = main.get_text("\n", strip=True)
    if not text:
        raise ValueError(f"no readable text found at {url}")
    return [TextPage(0, text)]


def section_for(text: str, current: str) -> str:
    for line in text.splitlines():
        candidate = " ".join(line.split())
        if len(candidate) <= 180 and re.match(r"^(?:\d+(?:\.\d+)*\.?\s+|[A-Z][A-Z\s/&()'’-]{7,})", candidate):
            return candidate
    return current


def split_text(text: str, max_chars: int = CHUNK_CHARS, overlap: int = OVERLAP_CHARS) -> list[str]:
    cleaned = re.sub(r"[\t\u00a0]+", " ", text)
    cleaned = re.sub(r"[ ]{2,}", " ", cleaned)
    cleaned = re.sub(r"\n{3,}", "\n\n", cleaned).strip()
    if not cleaned:
        return []
    chunks: list[str] = []
    start = 0
    while start < len(cleaned):
        end = min(start + max_chars, len(cleaned))
        if end < len(cleaned):
            boundary = max(cleaned.rfind("\n\n", start, end), cleaned.rfind(". ", start, end))
            if boundary > start + max_chars // 2:
                end = boundary + (2 if cleaned[boundary:boundary + 2] == ". " else 0)
        part = cleaned[start:end].strip()
        if part:
            chunks.append(part)
        if end >= len(cleaned):
            break
        start = max(start + 1, end - overlap)
    return chunks


def make_chunks(pages: Iterable[TextPage]) -> list[Chunk]:
    result: list[Chunk] = []
    current_section = ""
    for page in pages:
        current_section = section_for(page.text, current_section)
        pieces = split_text(page.text)
        for piece in pieces:
            current_section = section_for(piece, current_section)
            result.append(Chunk(
                index=len(result), page_start=page.number, page_end=page.number,
                section=current_section, content=piece,
                token_count=max(1, len(piece) // 4),
            ))
    return result


def gemini_embeddings(texts: list[str], model: str, api_key: str, dimensions: int) -> list[list[float]]:
    endpoint = f"https://generativelanguage.googleapis.com/v1beta/models/{model}:batchEmbedContents"
    request_items = [{
        "model": f"models/{model}",
        "content": {"parts": [{"text": text}]},
        "taskType": "RETRIEVAL_DOCUMENT",
        "outputDimensionality": dimensions,
    } for text in texts]
    for attempt in range(4):
        response = HTTP.post(endpoint, params={"key": api_key}, json={"requests": request_items}, timeout=(10, 60))
        if response.status_code == 429 or response.status_code >= 500:
            if attempt < 3:
                time.sleep(2 ** attempt)
                continue
        response.raise_for_status()
        vectors = [item["values"] for item in response.json().get("embeddings", [])]
        if len(vectors) != len(texts) or any(len(v) != dimensions for v in vectors):
            raise ValueError("Gemini returned an unexpected number of embeddings or dimensions")
        return vectors
    raise RuntimeError("Gemini embedding retries exhausted")


def vector_literal(values: list[float]) -> str:
    return "[" + ",".join(f"{value:.8f}" for value in values) + "]"


def ingest_one(conn: psycopg.Connection, source: dict[str, str], api_key: str, model: str, dimensions: int) -> tuple[str, int]:
    body, content_type = download(source["url"])
    checksum = hashlib.sha256(body).hexdigest()
    existing = conn.execute("SELECT id FROM documents WHERE checksum = %s", (checksum,)).fetchone()
    if existing:
        return "unchanged", 0

    pages = parse_document(body, content_type, source["url"])
    chunks = make_chunks(pages)
    if not chunks:
        raise ValueError("source parsed but produced no text chunks")
    parsed_url = urlparse(source["url"])
    filename = Path(parsed_url.path).name or (parsed_url.netloc + ".html")

    with conn.transaction():
        document_id = conn.execute(
            """INSERT INTO documents (title, doc_type, source_url, filename, total_pages, checksum, embedding_model)
               VALUES (%s, %s, %s, %s, %s, %s, %s) RETURNING id""",
            (source["title"], source["doc_type"], source["url"], filename,
             max((page.number for page in pages), default=0), checksum, model),
        ).fetchone()[0]

        for offset in range(0, len(chunks), EMBED_BATCH_SIZE):
            batch = chunks[offset:offset + EMBED_BATCH_SIZE]
            embeddings = gemini_embeddings([chunk.content for chunk in batch], model, api_key, dimensions)
            for chunk, embedding in zip(batch, embeddings, strict=True):
                conn.execute(
                    """INSERT INTO chunks
                       (document_id, chunk_index, page_start, page_end, section, content, token_count, embedding)
                       VALUES (%s, %s, %s, %s, %s, %s, %s, %s::vector)""",
                    (document_id, chunk.index, chunk.page_start, chunk.page_end, chunk.section,
                     chunk.content, chunk.token_count, vector_literal(embedding)),
                )
    return "ingested", len(chunks)


def main() -> int:
    load_dotenv(ROOT / ".env")
    database_url = os.getenv("DATABASE_URL")
    api_key = os.getenv("GEMINI_API_KEY")
    model = os.getenv("EMBEDDING_MODEL", "gemini-embedding-001")
    dimensions = int(os.getenv("EMBEDDING_DIM", "768"))
    if not database_url or not api_key:
        print("DATABASE_URL and GEMINI_API_KEY are required", file=sys.stderr)
        return 2
    failures = 0
    with psycopg.connect(database_url) as conn:
        for source in load_sources():
            try:
                status, count = ingest_one(conn, source, api_key, model, dimensions)
                print(f"{status}: {source['title']} ({count} chunks)")
            except Exception as exc:  # continue so one inaccessible source does not block others
                failures += 1
                print(f"failed: {source['title']}: {exc}", file=sys.stderr)
    print(f"completed with {failures} failed source(s)")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
