CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
    email TEXT NOT NULL CHECK (length(email) <= 320),
    phone TEXT CHECK (length(phone) <= 50),
    company TEXT CHECK (length(company) <= 200),
    status TEXT NOT NULL DEFAULT 'lead' CHECK (status IN ('lead', 'prospect', 'customer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contacts_email_lower_uq ON contacts (lower(email));
CREATE INDEX contacts_created_at_idx ON contacts (created_at DESC);

CREATE TABLE opportunities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(trim(title)) BETWEEN 1 AND 200),
    value_rm NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (value_rm >= 0),
    stage TEXT NOT NULL DEFAULT 'new' CHECK (stage IN ('new', 'contacted', 'qualified', 'proposal', 'won', 'lost')),
    position INTEGER NOT NULL DEFAULT 0,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((stage IN ('won', 'lost')) = (closed_at IS NOT NULL))
);
CREATE INDEX opportunities_contact_id_idx ON opportunities (contact_id);
CREATE INDEX opportunities_stage_position_idx ON opportunities (stage, position);

CREATE TABLE activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL CHECK (type IN ('call', 'email', 'meeting', 'note')),
    contact_id UUID REFERENCES contacts(id) ON DELETE CASCADE,
    opportunity_id UUID REFERENCES opportunities(id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL,
    description TEXT NOT NULL CHECK (length(trim(description)) BETWEEN 1 AND 5000),
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'system', 'meeting_extractor')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (contact_id IS NOT NULL OR opportunity_id IS NOT NULL)
);
CREATE INDEX activities_contact_occurred_idx ON activities (contact_id, occurred_at DESC);
CREATE INDEX activities_opportunity_idx ON activities (opportunity_id);

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    doc_type TEXT NOT NULL CHECK (doc_type IN ('e-invoice', 'sst', 'income-tax', 'other')),
    source_url TEXT NOT NULL,
    filename TEXT NOT NULL,
    total_pages INTEGER NOT NULL DEFAULT 0 CHECK (total_pages >= 0),
    checksum TEXT NOT NULL UNIQUE,
    embedding_model TEXT NOT NULL,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chunks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    page_start INTEGER NOT NULL DEFAULT 0,
    page_end INTEGER NOT NULL DEFAULT 0,
    section TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL DEFAULT 0,
    embedding VECTOR(768) NOT NULL,
    search_vector TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
    UNIQUE (document_id, chunk_index)
);
CREATE INDEX chunks_document_idx ON chunks (document_id, chunk_index);
CREATE INDEX chunks_search_vector_idx ON chunks USING GIN (search_vector);
CREATE INDEX chunks_embedding_hnsw_idx ON chunks USING hnsw (embedding vector_cosine_ops);

CREATE TABLE meeting_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_text TEXT NOT NULL,
    extracted JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'confirmed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
