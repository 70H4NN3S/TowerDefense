-- Global leaderboard: rank every player by trophies descending.
-- Ties are broken by user_id ascending for a stable, deterministic ordering.
-- REFRESH MATERIALIZED VIEW CONCURRENTLY requires a unique index, which is why
-- user_id is covered here.
CREATE MATERIALIZED VIEW global_leaderboard AS
SELECT
    RANK() OVER (ORDER BY trophies DESC, user_id ASC) AS rank,
    user_id,
    trophies
FROM  profiles
WITH NO DATA;

-- Required for REFRESH MATERIALIZED VIEW CONCURRENTLY (needs a unique index).
CREATE UNIQUE INDEX uq_global_leaderboard_user ON global_leaderboard (user_id);
-- Fast seek by rank for cursor-based pagination.
CREATE INDEX idx_global_leaderboard_rank ON global_leaderboard (rank);

-- Alliance leaderboard: sum each alliance's member trophies.
CREATE MATERIALIZED VIEW alliance_leaderboard AS
SELECT
    a.id   AS alliance_id,
    a.name AS alliance_name,
    a.tag  AS alliance_tag,
    COALESCE(SUM(p.trophies), 0) AS total_trophies,
    COUNT(am.user_id)            AS member_count
FROM  alliances a
JOIN  alliance_members am ON am.alliance_id = a.id
JOIN  profiles p          ON p.user_id      = am.user_id
GROUP BY a.id, a.name, a.tag
WITH NO DATA;

-- Required for CONCURRENTLY refresh.
CREATE UNIQUE INDEX uq_alliance_leaderboard_alliance ON alliance_leaderboard (alliance_id);
-- Fast ordering for pagination.
CREATE INDEX idx_alliance_leaderboard_trophies ON alliance_leaderboard (total_trophies DESC);

-- Populate both views immediately so the first read returns data.
REFRESH MATERIALIZED VIEW global_leaderboard;
REFRESH MATERIALIZED VIEW alliance_leaderboard;
