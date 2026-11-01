BEGIN;
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS memory_id UUID REFERENCES memories(id) ON DELETE SET NULL;
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS original_name TEXT NOT NULL DEFAULT 'Attachment';
ALTER TABLE media_assets ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'upload' CHECK (source IN ('upload', 'voice'));
CREATE INDEX IF NOT EXISTS assets_memory_pending_idx ON media_assets(memory_id, status);

CREATE TABLE IF NOT EXISTS media_deletions (
    object_key TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
--here every detach, including a cascading memory delete, queues a physical cleanup.
--postgresql commits removal immediately; object deletion is retried by a worker.
CREATE OR REPLACE FUNCTION queue_detached_media() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN 
    IF NOT EXISTS (SELECT 1 FROM memory_assets WHERE asset_id=OLD.asset_id) THEN
        INSERT INTO media_deletions(object_key)
            SELECT object_key FROM media_assets WHERE id=OLD.asset_id
            ON CONFLICT DO NOTHING;
        UPDATE media_assets SET status='deleted' WHERE id=OLD.asset_id;
    END IF;
    RETURN OLD;
END;
$$;
DROP TRIGGER IF EXISTS memory_asset_cleanup ON memory_assets;
CREATE TRIGGER memory_asset_cleanup AFTER DELETE ON memory_assets
    FOR EACH ROW EXECUTIVE FUNCTION queue_detached_media();
COMMIT;