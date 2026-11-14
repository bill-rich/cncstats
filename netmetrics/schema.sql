-- Zulu GO network metrics, filled by netmetrics.py from the per match client
-- logs in the cncstats logs volume. Times are UTC. t_s is seconds since the
-- reporter's match start line.
CREATE DATABASE IF NOT EXISTS zulumetrics CHARACTER SET utf8mb4;
USE zulumetrics;

CREATE TABLE IF NOT EXISTS matches (
  match_id       BIGINT UNSIGNED PRIMARY KEY,   -- the game seed = log bucket name
  started_at     DATETIME NOT NULL,
  ended_at       DATETIME NOT NULL,
  map            VARCHAR(128),
  humans         TINYINT,
  router_slot    TINYINT,
  router_name    VARCHAR(64),
  version        VARCHAR(32),
  duration_s     INT,
  frames         INT,
  sim_fps        FLOAT,          -- logic frames per wall second, 60 is full speed
  desync         TINYINT,
  reports        TINYINT,        -- how many players' logs arrived
  frozen_s       FLOAT,          -- worst player's total stall time
  worst_stall_ms INT,
  disc_screens   TINYINT,
  slowest_name   VARCHAR(64),    -- PC that held the game speed down most (2.1.6+)
  felt_s         FLOAT,          -- worst player's felt freeze: time past the first 100 ms of each wait
  INDEX (started_at)
);

CREATE TABLE IF NOT EXISTS match_players (
  match_id       BIGINT UNSIGNED,
  slot           TINYINT,
  name           VARCHAR(64),
  reported       BOOL,           -- false: in the roster but no log arrived
  version        VARCHAR(32),
  finished       BOOL,           -- the log reached Match end
  end_frame      INT,
  stall_n        INT,
  stall_s        FLOAT,
  stall_max_ms   INT,
  disc_screens   INT,
  disc_s         FLOAT,
  lat_ms         INT,            -- average lockstep latency
  min_fps        INT,            -- lowest game speed this player saw
  resend_req     INT,
  resend_ans     INT,
  route_samples  INT,
  relayed_share  FLOAT,
  ping_ms        INT,            -- average GNS ping over all routes
  rep_fps        FLOAT,          -- average fps this PC reported to the router (2.1.6+)
  held_back      FLOAT,          -- share of time this PC was the slowest and under 54 fps
  client_ms      FLOAT,          -- average client update (input, UI, drawing) per frame
  logic_ms       FLOAT,          -- average logic update per logic frame
  max_objs       INT,
  lag_caused     FLOAT,          -- seconds of felt freeze this player caused, from the router's view (lag_caused)
  felt_stall_s   FLOAT,          -- this player's waits past the first 100 ms each (stall_s counts every ms)
  ping_mid_ms    INT,            -- the player's middle link ping (link_metrics), as on the profile
  direct_share   FLOAT,          -- share of links not relayed by this player's fault (link_metrics)
  PRIMARY KEY (match_id, slot)
);

CREATE TABLE IF NOT EXISTS stalls (
  match_id       BIGINT UNSIGNED,
  reporter       VARCHAR(64),
  reporter_slot  TINYINT,
  at             DATETIME,
  t_s            INT,
  frame          INT,
  waited_ms      INT,
  on_mask        SMALLINT,       -- slots whose frame was missing
  blame_mask     SMALLINT,       -- router's view when it sent one, else on_mask
  blamed         VARCHAR(64),    -- name when one slot is blamed, else 'several'
  INDEX (match_id), INDEX (at)
);

CREATE TABLE IF NOT EXISTS netstat (
  match_id       BIGINT UNSIGNED,
  slot           TINYINT,
  name           VARCHAR(64),
  at             DATETIME,
  t_s            INT,
  frame          INT,
  fps            SMALLINT,
  runahead       SMALLINT,
  lat_ms         INT,
  stall_n        INT,
  stall_ms       INT,
  stall_max_ms   INT,
  rreq           INT,
  rans           INT,
  client_ms      FLOAT,
  client_max_ms  INT,
  logic_ms       FLOAT,
  logic_max_ms   INT,
  objs           INT,
  INDEX (match_id), INDEX (at)
);

CREATE TABLE IF NOT EXISTS routes (
  match_id       BIGINT UNSIGNED,
  slot           TINYINT,
  name           VARCHAR(64),
  peer           VARCHAR(64),
  at             DATETIME,
  t_s            INT,
  relayed        BOOL,
  ping_ms        INT,
  pending        INT,
  out_kbps       INT,            -- upload rate on this link (2.1.6+)
  queue_ms       INT,            -- how long a new message waits to go out
  INDEX (match_id), INDEX (at), INDEX (name, peer)
);

-- The packet router's view of each PC's fps (2.1.6+). The lowest one is the
-- speed the whole game runs at.
CREATE TABLE IF NOT EXISTS peer_fps (
  match_id       BIGINT UNSIGNED,
  slot           TINYINT,
  name           VARCHAR(64),
  at             DATETIME,
  t_s            INT,
  lat_ms         INT,
  fps            SMALLINT,
  INDEX (match_id), INDEX (at)
);

-- Columns added after the first deploy. Inserts are positional, so new columns
-- only ever go on the end, here and in the CREATE above.
ALTER TABLE matches ADD COLUMN IF NOT EXISTS slowest_name VARCHAR(64), ADD COLUMN IF NOT EXISTS felt_s FLOAT;
ALTER TABLE match_players ADD COLUMN IF NOT EXISTS rep_fps FLOAT, ADD COLUMN IF NOT EXISTS held_back FLOAT,
  ADD COLUMN IF NOT EXISTS client_ms FLOAT, ADD COLUMN IF NOT EXISTS logic_ms FLOAT, ADD COLUMN IF NOT EXISTS max_objs INT,
  ADD COLUMN IF NOT EXISTS lag_caused FLOAT, ADD COLUMN IF NOT EXISTS felt_stall_s FLOAT,
  ADD COLUMN IF NOT EXISTS ping_mid_ms INT, ADD COLUMN IF NOT EXISTS direct_share FLOAT;
ALTER TABLE netstat ADD COLUMN IF NOT EXISTS client_ms FLOAT, ADD COLUMN IF NOT EXISTS client_max_ms INT,
  ADD COLUMN IF NOT EXISTS logic_ms FLOAT, ADD COLUMN IF NOT EXISTS logic_max_ms INT, ADD COLUMN IF NOT EXISTS objs INT;
ALTER TABLE routes ADD COLUMN IF NOT EXISTS out_kbps INT, ADD COLUMN IF NOT EXISTS queue_ms INT;

-- One row per pair of players per game, both directions merged (link_metrics):
-- the relayed share, the ping, and whose fault the relay was, as the profile counts it.
CREATE TABLE IF NOT EXISTS link_pairs (
  match_id       BIGINT UNSIGNED,
  a              VARCHAR(64),
  b              VARCHAR(64),
  relayed        FLOAT,          -- share of samples relayed, both directions
  ping_ms        INT,
  charged_to     VARCHAR(64),    -- a, b, 'both' (a tie) or NULL (not relayed)
  INDEX (match_id), INDEX (a, b)
);

-- Each player's form over their last 5 games, rebuilt every run (compute_forms),
-- read by the GO Services for the in-game player profile.
CREATE TABLE IF NOT EXISTS player_form (
  name           VARCHAR(64) PRIMARY KEY,
  form           TEXT NOT NULL,     -- JSON, see compute_forms
  updated_at     DATETIME
);

-- Single holder honors from radarvan stats, replaced daily by honors.py;
-- netmetrics.py copies each player's list into their player_form.
CREATE TABLE IF NOT EXISTS player_honors (
  name           VARCHAR(64) PRIMARY KEY,
  honors         TEXT NOT NULL,   -- JSON [{"k": key, "why": text}]
  updated_at     DATETIME
);

CREATE TABLE IF NOT EXISTS ingest_state (
  bucket         BIGINT UNSIGNED PRIMARY KEY,
  newest_mtime   BIGINT,
  parser         SMALLINT,
  ingested_at    DATETIME
);

-- Who caused the stalls: each blamed player's stall seconds, as seen by the
-- reporter who suffered most from them (reporters see the same freeze, so a
-- sum across reporters would count it once per player).
CREATE OR REPLACE VIEW blame AS
SELECT match_id, blamed, MAX(s) AS stall_s, MAX(n) AS stalls
FROM (SELECT match_id, reporter, blamed, SUM(waited_ms) / 1000 AS s, COUNT(*) AS n
      FROM stalls WHERE reporter <> blamed GROUP BY match_id, reporter, blamed) x
GROUP BY match_id, blamed;
