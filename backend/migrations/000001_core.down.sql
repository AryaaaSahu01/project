-- DESTRUCTIVE: only use on disposable local databases, never a live user database.
DROP TABLE IF EXISTS return_events;
DROP TABLE IF EXISTS memory_links;
DROP TABLE IF EXISTS memory_assets;
DROP TABLE IF EXISTS media_assets;
DROP TABLE IF EXISTS reflections;
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS workspace_invitations;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS users;
