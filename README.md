# taxPOD Mini CRM backend

The backend uses Go, Gin, PostgreSQL, and pgvector, following the project design document. Its REST API uses the `/api/v1` prefix and includes core CRM endpoints, a Gemini-backed tax RAG endpoint, and offline document ingestion.

## Local setup

1. Copy `.env.example` to `.env` and set `GEMINI_API_KEY` when the RAG integration is added. Change `API_PORT`, `DB_PORT`, or `CORS_ORIGINS` if any configured port is occupied. The current defaults are API `18080`, database `15432`, and frontend origin `15173`.
2. Start PostgreSQL with pgvector from this folder:

   ```powershell
   docker compose up -d db
   ```

3. Start the API from the `api` folder (Go 1.22+):

   ```powershell
   go run ./cmd/server
   ```

The API applies embedded SQL migrations on startup. Check `http://localhost:18080/healthz` or `http://localhost:18080/api/v1/healthz` for service and database status.

## Tax RAG document ingestion

The source list is in `ingest/sources.yaml`. It currently contains five public LHDN/RMCD sources: the e-Invoice Guideline v4.8, e-Invoice Specific Guideline v4.9, e-Invoice FAQs, RMCD's Sales Tax Act registration guide, and RMCD's Service Tax FAQ. The ingestion script downloads each source, extracts PDF pages or readable HTML, chunks text at about 700 tokens with overlap, calls Gemini `gemini-embedding-001` using the `RETRIEVAL_DOCUMENT` task type, then writes document and chunk metadata plus 768-dimensional vectors to PostgreSQL/pgvector. It skips unchanged content by SHA-256 checksum and continues after an individual source failure.

Install Python 3.11+ dependencies and ingest:

```powershell
python -m venv ingest\.venv
ingest\.venv\Scripts\Activate.ps1
pip install -r ingest\requirements.txt
python ingest\ingest.py
```

Set `GEMINI_API_KEY` in the root `.env` before ingestion. The API uses the same key for `gemini-embedding-001` query embeddings and the configured Gemini Flash generation model. For example:

```http
POST /api/v1/rag/ask
Content-Type: application/json

{"question":"What is the e-Invoice exemption threshold?","doc_type":"e-invoice"}
```

`GET /api/v1/rag/documents` lists ingested sources. An answer below the configured similarity threshold, or one whose citations fail server-side validation, returns the fixed refusal message. The RAG API returns document title, page range/section, and source snippets for every accepted citation.

## CRM API routes

- `GET/POST /api/v1/contacts`; `GET/PUT/DELETE /api/v1/contacts/:id`
- `GET/POST /api/v1/opportunities`; `PUT/DELETE /api/v1/opportunities/:id`; `PATCH /api/v1/opportunities/:id/stage`
- `GET/POST /api/v1/activities`
- `GET /api/v1/dashboard`

Contact list filters are `q`, `status`, `page`, and `page_size`. Opportunity lists accept `stage` and `contact_id`; activity lists accept `contact_id` and `opportunity_id`. Money values are represented as decimal strings in JSON. System activities are written transactionally when contacts are created/updated, opportunities are created/updated, or an opportunity changes stage.

## Configuration

The API reads `.env` from the repository root when started from `api/`, and also accepts ordinary process environment variables. Never commit real API keys.

| Variable | Purpose |
| --- | --- |
| `API_PORT` | HTTP listen port |
| `DB_PORT` | Host port for local PostgreSQL |
| `DATABASE_URL` | PostgreSQL connection string |
| `CORS_ORIGINS` | Comma-separated frontend origins |
| `GEMINI_API_KEY` | Gemini key for real embedding and generation calls |

