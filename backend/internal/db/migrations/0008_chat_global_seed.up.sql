-- Seed the single global chat channel. The ID is fixed so application code
-- can reference it as chat.GlobalChannelID without querying the database.
INSERT INTO chat_channels (id, kind, created_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'global', now());
