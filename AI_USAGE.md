# AI usage

## Tool and scope

OpenAI Codex was used in this development session to read the provided PRD and technical design, scaffold the Go backend, and draft the CRM, RAG, ingestion, and evaluation code. This record covers this session; add any other tools used during the assessment before submission.

## Example

**Actual user prompt:**

> “先从后端开始做吧，记得一定要跟着我们的计划走！”

The prompt was given with the assessment brief in the conversation, including the instruction to prioritize a real Malaysian tax RAG pipeline with citations, abstention, and retrieval evaluation before polishing optional features.

**Codex output (summary):**

Codex created a Go/Gin backend scaffold, a pgvector schema, REST handlers for contacts, opportunities, activities, dashboard, and RAG, a Gemini REST client, a Python public-document ingestion command, and a ten-question retrieval evaluation set.

**Changes and checks made during this session:**

- Changed the sample ports from the design document's `8080`/`5173` to configurable defaults `18080`/`15173` after the user said those ports were already occupied; added database host port `15432` and documented the matching CORS/API settings.
- Used the current public LHDN e-Invoice Guideline v4.8 and Specific Guideline v4.9 URLs plus RMCD sources in the ingestion list.
- Moved shared RAG result types into the model package after the first compile found a repository/service import cycle.
- Rechecked the contact/opportunity/activity foreign-key shape against the brief and made opportunity ownership mandatory while allowing an activity to target either a contact or an opportunity.
- Compiled the Go server and evaluation command and syntax-checked the Python ingester. Unit or integration tests were not run, the documents were not ingested, and no evaluation score is claimed.

## Where AI helped most

It helped translate the design document into a coherent first implementation across route wiring, database schema, Gemini request/response structures, and the ingestion/evaluation scaffolding.

## Where AI was less reliable

The first generated code layout had a Go package dependency cycle, caught by compilation. The generated SQL and API behavior also need a live PostgreSQL review, and the retrieval threshold and golden-set hit rate cannot be validated until the corpus is imported and the evaluation is run with a real Gemini API key.
