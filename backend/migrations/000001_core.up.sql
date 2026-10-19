-- C5.1: normalized persistent foundation for accounts, collaboration, memories and media.
-- Existing in-memory Go repositories are not yet switched over (C5.2-C5.5).
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    email_verified_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_length CHECK (char_length(email) BETWEEN 3 AND 254),
    CONSTRAINT users_email_lowercase CHECK (email = lower(btrim(email)))
);
CREATE UNIQUE INDEX users_email_unique_idx ON users (email);

CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT profile_name_length CHECK (char_length(display_name) <= 100),
    CONSTRAINT profile_bio_length CHECK (char_length(bio) <= 2000)
);

-- Sessions contain only a SHA-256 hash of the opaque cookie token, never the token itself.
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT session_hash_length CHECK (octet_length(token_hash) = 32)
);
CREATE INDEX sessions_active_user_idx ON sessions (user_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('personal', 'shared')),
    name TEXT NOT NULL,
    owner_user_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT workspace_name_length CHECK (char_length(name) BETWEEN 1 AND 120)
);
CREATE INDEX workspaces_owner_idx ON workspaces (owner_user_id);

CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id, workspace_id);
CREATE UNIQUE INDEX workspace_single_owner_idx ON workspace_members (workspace_id) WHERE role = 'owner';

CREATE TABLE workspace_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    invited_by UUID NOT NULL REFERENCES users(id),
    invitee_email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('editor', 'viewer')),
    token_hash BYTEA NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT invitation_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT invitation_email_normalized CHECK (invitee_email = lower(btrim(invitee_email)))
);
CREATE INDEX invitations_workspace_status_idx ON workspace_invitations (workspace_id, status);

-- One stable memory ID remains in use from draft through sealing and return.
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sealed', 'returned')),
    sealed_at TIMESTAMPTZ,
    unlock_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id),
    CONSTRAINT memory_title_length CHECK (char_length(title) <= 500),
    CONSTRAINT memory_seal_dates CHECK (
      (status = 'draft' AND sealed_at IS NULL AND unlock_at IS NULL)
      OR (status IN ('sealed','returned') AND sealed_at IS NOT NULL AND unlock_at > sealed_at)
    )
);
CREATE INDEX memories_workspace_timeline_idx ON memories (workspace_id, created_at DESC, id);
CREATE INDEX memories_due_idx ON memories (unlock_at, id) WHERE status = 'sealed';

CREATE TABLE reflections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    memory_id UUID NOT NULL,
    author_user_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sealed')),
    sealed_child_memory_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id),
    FOREIGN KEY (memory_id, workspace_id) REFERENCES memories(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (sealed_child_memory_id, workspace_id) REFERENCES memories(id, workspace_id),
    CONSTRAINT reflection_child_differs CHECK (sealed_child_memory_id IS NULL OR sealed_child_memory_id <> memory_id),
    CONSTRAINT sealed_reflection_has_child CHECK ((status = 'draft' AND sealed_child_memory_id IS NULL) OR (status = 'sealed' AND sealed_child_memory_id IS NOT NULL))
);
CREATE INDEX reflections_parent_idx ON reflections (workspace_id, memory_id, created_at);

CREATE TABLE media_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    uploaded_by UUID NOT NULL REFERENCES users(id),
    object_key TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL CHECK (kind IN ('image', 'video', 'audio')),
    content_type TEXT NOT NULL,
    declared_bytes BIGINT NOT NULL CHECK (declared_bytes > 0),
    verified_bytes BIGINT CHECK (verified_bytes >= 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'rejected', 'deleted')),
    duration_ms INTEGER CHECK (duration_ms >= 0),
    width_px INTEGER CHECK (width_px > 0),
    height_px INTEGER CHECK (height_px > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    verified_at TIMESTAMPTZ,
    UNIQUE (id, workspace_id)
);
CREATE INDEX assets_workspace_status_idx ON media_assets (workspace_id, status, created_at DESC);

CREATE TABLE memory_assets (
    workspace_id UUID NOT NULL,
    memory_id UUID NOT NULL,
    asset_id UUID NOT NULL,
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    attached_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (memory_id, asset_id),
    FOREIGN KEY (memory_id, workspace_id) REFERENCES memories(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (asset_id, workspace_id) REFERENCES media_assets(id, workspace_id)
);
CREATE INDEX memory_assets_asset_idx ON memory_assets (asset_id);

-- Independent relationship graph, separate from reflection-child lineage.
CREATE TABLE memory_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    from_memory_id UUID NOT NULL,
    to_memory_id UUID NOT NULL,
    relation TEXT NOT NULL CHECK (relation IN ('related', 'continues', 'same_event', 'inspired_by')),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (from_memory_id, workspace_id) REFERENCES memories(id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (to_memory_id, workspace_id) REFERENCES memories(id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT link_no_self_reference CHECK (from_memory_id <> to_memory_id),
    UNIQUE (from_memory_id, to_memory_id, relation)
);
CREATE INDEX memory_links_from_idx ON memory_links (workspace_id, from_memory_id);
CREATE INDEX memory_links_to_idx ON memory_links (workspace_id, to_memory_id);

CREATE TABLE return_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    memory_id UUID NOT NULL,
    kind TEXT NOT NULL DEFAULT 'capsule_due' CHECK (kind IN ('capsule_due')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivered', 'cancelled')),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    FOREIGN KEY (memory_id, workspace_id) REFERENCES memories(id, workspace_id) ON DELETE CASCADE,
    UNIQUE (memory_id, kind)
);
CREATE INDEX return_events_pending_idx ON return_events (recorded_at, id) WHERE status = 'pending';
