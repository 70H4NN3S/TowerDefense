-- Seed the tower catalog with five starter towers.
--
-- Stats are defined at every level (1-10). The formula is roughly:
--   stat_at_level_n = base * (1 + 0.15 * (n - 1))
-- rounded to the nearest integer, giving ~15 % growth per level.
--
-- gold_cost at level n is the cost to upgrade *from* level n-1 to level n.
-- Level 1 rows have gold_cost = 0 (the tower is obtained via diamonds).

-- ── 1. Archer Tower (common) ─────────────────────────────────────────────────
-- Balanced damage, medium range, fast rate. Cheapest entry point.
WITH t AS (
    INSERT INTO tower_templates (id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)
    VALUES (
        '11111111-1111-4111-8111-111111111111',
        'Archer Tower', 'common', 20, 150, 10, 50,
        'A reliable all-rounder. Fast attack speed with solid range.'
    )
    RETURNING id
)
INSERT INTO tower_levels (template_id, level, gold_cost, damage, range_, rate)
SELECT t.id, lvl, gold_cost, damage, range_, rate
FROM t,
(VALUES
    (1,    0,  20, 150, 10),
    (2,  200,  23, 172, 11),
    (3,  400,  26, 198, 12),
    (4,  700,  30, 228, 14),
    (5, 1100,  35, 262, 16),
    (6, 1600,  40, 301, 18),
    (7, 2200,  46, 346, 21),
    (8, 2900,  53, 398, 24),
    (9, 3700,  61, 458, 27),
    (10,4700,  70, 527, 31)
) AS vals(lvl, gold_cost, damage, range_, rate);

-- ── 2. Cannon Tower (rare) ───────────────────────────────────────────────────
-- High damage per shot, short range, slow rate. Anti-tank specialist.
WITH t AS (
    INSERT INTO tower_templates (id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)
    VALUES (
        '22222222-2222-4222-8222-222222222222',
        'Cannon Tower', 'rare', 80, 100, 4, 150,
        'Devastating single-target shots. Slow but hits hard.'
    )
    RETURNING id
)
INSERT INTO tower_levels (template_id, level, gold_cost, damage, range_, rate)
SELECT t.id, lvl, gold_cost, damage, range_, rate
FROM t,
(VALUES
    (1,    0,  80, 100,  4),
    (2,  400,  92, 115,  4),
    (3,  800, 106, 132,  5),
    (4, 1400, 122, 152,  5),
    (5, 2100, 140, 175,  6),
    (6, 3000, 161, 201,  7),
    (7, 4100, 185, 231,  8),
    (8, 5400, 213, 266,  9),
    (9, 6900, 245, 306, 10),
    (10,8700, 282, 352, 12)
) AS vals(lvl, gold_cost, damage, range_, rate);

-- ── 3. Frost Tower (rare) ────────────────────────────────────────────────────
-- Low damage but exceptional range. Weakens enemies for follow-up towers.
WITH t AS (
    INSERT INTO tower_templates (id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)
    VALUES (
        '33333333-3333-4333-8333-333333333333',
        'Frost Tower', 'rare', 15, 200, 7, 150,
        'Long-range ice blasts that slow the advance.'
    )
    RETURNING id
)
INSERT INTO tower_levels (template_id, level, gold_cost, damage, range_, rate)
SELECT t.id, lvl, gold_cost, damage, range_, rate
FROM t,
(VALUES
    (1,    0,  15, 200,  7),
    (2,  350,  17, 230,  8),
    (3,  700,  20, 265,  9),
    (4, 1200,  23, 305, 10),
    (5, 1800,  26, 351, 11),
    (6, 2600,  30, 404, 13),
    (7, 3500,  35, 465, 15),
    (8, 4700,  40, 535, 17),
    (9, 6000,  46, 615, 19),
    (10,7600,  53, 708, 22)
) AS vals(lvl, gold_cost, damage, range_, rate);

-- ── 4. Lightning Tower (epic) ────────────────────────────────────────────────
-- Blisteringly fast attack rate with good damage. Premium tower.
WITH t AS (
    INSERT INTO tower_templates (id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)
    VALUES (
        '44444444-4444-4444-8444-444444444444',
        'Lightning Tower', 'epic', 35, 130, 18, 300,
        'Rapid-fire electrical bursts that shred swarms.'
    )
    RETURNING id
)
INSERT INTO tower_levels (template_id, level, gold_cost, damage, range_, rate)
SELECT t.id, lvl, gold_cost, damage, range_, rate
FROM t,
(VALUES
    (1,    0,  35, 130, 18),
    (2,  600,  40, 150, 21),
    (3, 1200,  46, 172, 24),
    (4, 2000,  53, 198, 27),
    (5, 3000,  61, 228, 31),
    (6, 4300,  70, 262, 36),
    (7, 5800,  81, 301, 41),
    (8, 7600,  93, 347, 47),
    (9, 9700, 107, 399, 54),
    (10,12200,123, 459, 62)
) AS vals(lvl, gold_cost, damage, range_, rate);

-- ── 5. Inferno Tower (legendary) ─────────────────────────────────────────────
-- Highest base damage, very short range, decent rate. The siege weapon.
WITH t AS (
    INSERT INTO tower_templates (id, name, rarity, base_damage, base_range, base_rate, cost_diamonds, description)
    VALUES (
        '55555555-5555-4555-8555-555555555555',
        'Inferno Tower', 'legendary', 150, 80, 6, 600,
        'Scorches anything within short range. Unmatched raw damage.'
    )
    RETURNING id
)
INSERT INTO tower_levels (template_id, level, gold_cost, damage, range_, rate)
SELECT t.id, lvl, gold_cost, damage, range_, rate
FROM t,
(VALUES
    (1,     0, 150, 80,  6),
    (2,  1000, 172, 92,  7),
    (3,  2000, 198, 106,  8),
    (4,  3500, 228, 122,  9),
    (5,  5500, 262, 140, 10),
    (6,  8000, 301, 161, 12),
    (7, 11000, 347, 185, 14),
    (8, 14500, 399, 213, 16),
    (9, 18500, 459, 245, 18),
    (10,23000, 528, 282, 21)
) AS vals(lvl, gold_cost, damage, range_, rate);
