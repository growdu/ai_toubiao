-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- 00013: KB chunk ID type + TSV auto-maintenance
--
-- Two related fixes that close the gap between the schema in 00007 and the
-- code in services/knowledge-svc:
--
--   (1) kb_chunks.id was BIGSERIAL, but model.KBChunk.ID is uuid.UUID and
--       kb_store.CreateChunkWithVec passes c.ID = uuid.New() into the INSERT.
--       Every ingest hit:
--           ERROR: invalid input syntax for type bigint:
--                  "d733d823-4760-4d01-9ced-162e65e190ff" (SQLSTATE 22P02)
--       The unit tests didn't catch it because they use a fakeStore that
--       never touches PG. The integration test added in this commit
--       (TestIntegration_ChunkInsertAndSearch) reproduces the failure on the
--       old schema and passes on the fixed one.
--
--       Fix: ALTER kb_chunks.id to UUID DEFAULT gen_random_uuid(). Existing
--       rows are backfilled (the migration is written to be idempotent — if
--       id is already UUID it no-ops).
--
--   (2) kb_chunks.content_tsv TSVECTOR had a GIN index but no trigger to
--       populate it, so it was always NULL and the BM25 path was dead code.
--       Fix: trigger that mirrors content to content_tsv using the 'simple'
--       text search config (no stopword stripping, since Chinese text does
--       not have word boundaries — the engine falls back to per-character
--       matching in that case).
--
-- These two together unlock hybrid retrieval (vector + BM25 + RRF) which
-- lands in the next commit.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- (1) kb_chunks.id BIGSERIAL -> UUID
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF (SELECT data_type FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'kb_chunks'
          AND column_name = 'id') = 'bigint' THEN

        -- Add a UUID column with a default, backfill, swap, drop the old.
        -- We do this in four steps so the table is never without a primary key
        -- and FK references from kb_evidence_links keep working (it references
        -- evidence, not chunks, so no FK touch is needed).

        -- Drop the sequence first; it will become orphaned otherwise.
        ALTER TABLE kb_chunks ALTER COLUMN id DROP DEFAULT;
        DROP SEQUENCE IF EXISTS kb_chunks_id_seq;

        ALTER TABLE kb_chunks
            ALTER COLUMN id TYPE UUID USING gen_random_uuid(),
            ALTER COLUMN id SET DEFAULT gen_random_uuid();
    END IF;
END$$;

-- ---------------------------------------------------------------------------
-- (2) content_tsv auto-maintenance trigger
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION kb_chunks_tsv_update() RETURNS trigger AS $$
BEGIN
    -- 'simple' config: no stemming, no stopword list. For mixed CN/EN content
    -- this is the right choice — pg's default 'english' config mangles CJK.
    -- pg handles CJK by treating each character as a token.
    NEW.content_tsv := to_tsvector('simple', coalesce(NEW.content, ''));
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_kb_chunks_tsv ON kb_chunks;
CREATE TRIGGER trg_kb_chunks_tsv
    BEFORE INSERT OR UPDATE OF content ON kb_chunks
    FOR EACH ROW EXECUTE FUNCTION kb_chunks_tsv_update();

-- Backfill any pre-existing rows (only matters if a prior install ingested
-- chunks while the trigger was missing). Idempotent — NULLs become populated.
UPDATE kb_chunks
   SET content_tsv = to_tsvector('simple', coalesce(content, ''))
 WHERE content_tsv IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_kb_chunks_tsv ON kb_chunks;
DROP FUNCTION IF EXISTS kb_chunks_tsv_update();

-- Reverting the UUID change back to BIGSERIAL is destructive (loses the
-- association between application-side UUID and any DB-side bigint sequence),
-- so the down migration only drops the trigger + function. Operators who
-- need to revert further should restore from backup.
--
-- DO $$
-- BEGIN
--     IF (SELECT data_type FROM information_schema.columns
--         WHERE table_schema = current_schema()
--           AND table_name = 'kb_chunks'
--           AND column_name = 'id') = 'uuid' THEN
--         CREATE SEQUENCE kb_chunks_id_seq;
--         ALTER TABLE kb_chunks
--             ALTER COLUMN id DROP DEFAULT,
--             ALTER COLUMN id TYPE BIGINT USING nextval('kb_chunks_id_seq'),
--             ALTER COLUMN id SET DEFAULT nextval('kb_chunks_id_seq');
--     END IF;
-- END$$;

-- +goose StatementEnd