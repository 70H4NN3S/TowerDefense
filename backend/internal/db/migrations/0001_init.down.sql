-- Dropping citext is only safe when no tables use citext columns.
-- Safe to run in a fresh test schema; exercise caution on a live database.
DROP EXTENSION IF EXISTS citext;
