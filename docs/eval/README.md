# RAG retrieval evaluation

`golden.json` contains nine answerable questions and one deliberately out-of-scope question. Every answerable question records its expected source title. The evaluation command reports source Hit@k and whether the out-of-scope question falls below the configured refusal threshold.

Run it after starting PostgreSQL, setting `GEMINI_API_KEY` in the repository root `.env`, and importing the sources:

```powershell
cd api
go run ./cmd/eval -set ../docs/eval/golden.json
```

The command uses the configured real Gemini query embedding model and the same pgvector retrieval repository as the API. It does not call the answer-generation model, so evaluation measures retrieval and threshold behavior without generation variability.

Record the actual output, retrieval hit rate, and any misses in `docs/eval/REPORT.md` after running the evaluation against an imported knowledge base. Do not report a score before ingestion and execution.
