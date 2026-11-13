#!/usr/bin/env python3
"""Writes grafana/dashboards/*.json. Edit the SQL here, rerun, redeploy."""
import json
import os

DS = {"type": "mysql", "uid": "zulumetrics"}
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "grafana", "dashboards")
MATCH_LINK = "/grafana/d/zm-match/match?var-match=${__data.fields.Match}&from=${__data.fields.from_ms}&to=${__data.fields.to_ms}"
HIDE = [{"matcher": {"id": "byRegexp", "options": "from_ms|to_ms"}, "properties": [{"id": "custom.hidden", "value": True}]}]
IN_RANGE = "m.started_at BETWEEN $__timeFrom() AND $__timeTo()"


def panel(title, sql, x, y, w, h, kind="table", fmt=None, unit=None, overrides=None, desc=None, style=None):
    p = {"title": title, "type": kind, "datasource": DS, "gridPos": {"x": x, "y": y, "w": w, "h": h},
         "targets": [{"refId": "A", "datasource": DS, "rawQuery": True, "editorMode": "code", "rawSql": sql,
                      "format": fmt or ("time_series" if kind == "timeseries" else "table")}],
         "fieldConfig": {"defaults": {"custom": {}}, "overrides": overrides or []}, "options": {}}
    if unit:
        p["fieldConfig"]["defaults"]["unit"] = unit
    if desc:
        p["description"] = desc
    if kind == "timeseries":
        p["fieldConfig"]["defaults"]["custom"] = {"drawStyle": style or "line", "pointSize": 6, "showPoints": "always" if style == "points" else "auto",
                                                  "spanNulls": True, "lineWidth": 1}
        p["options"] = {"legend": {"displayMode": "table", "placement": "right", "calcs": ["mean", "max"]},
                        "tooltip": {"mode": "multi", "sort": "desc"}}
    if kind == "stat":
        p["options"] = {"reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False}, "colorMode": "none", "graphMode": "none"}
    if kind == "table":
        p["options"] = {"showHeader": True, "cellHeight": "sm"}
    return p


def dashboard(uid, title, panels, time_from="now-30d", templating=None, desc=""):
    for i, p in enumerate(panels):
        p["id"] = i + 1
    return {"uid": uid, "title": title, "description": desc, "tags": ["zulu"], "timezone": "browser", "schemaVersion": 39,
            "time": {"from": time_from, "to": "now"}, "refresh": "", "editable": True,
            "templating": {"list": templating or []}, "panels": panels,
            "links": [{"type": "dashboards", "tags": ["zulu"], "asDropdown": False, "title": "Zulu"}]}


LAG_DESC = ("Freeze time each player caused, as the player's profile counts it. The packet router sees everyone directly, so who it"
            " waited on is charged to them; when most other players froze and the router did not, the router's relay is charged;"
            " when one player froze alone, that player's link. Games of 3+ players with the router's log only (in a 1v1 nobody"
            " can tell whose end of the link is bad). Only the part of each wait past its first 100 ms counts.")

overview = dashboard("zm-overview", "Game nights", [
    panel("Games", "SELECT COUNT(*) FROM matches m WHERE " + IN_RANGE, 0, 0, 4, 4, "stat"),
    panel("Game speed (fps, 60 = full)", "SELECT ROUND(SUM(frames)/SUM(duration_s),1) FROM matches m WHERE " + IN_RANGE, 4, 0, 5, 4, "stat"),
    panel("Frozen minutes", "SELECT ROUND(SUM(felt_s)/60,1) FROM matches m WHERE " + IN_RANGE, 9, 0, 5, 4, "stat",
          desc="Per game, the felt freeze time of the player who froze the most, summed over games. Only the part of each wait past its first 100 ms counts:"
               " waits of a few ms slow the game down but are not felt as freezes."),
    panel("Freezes of 5 s or more", "SELECT COUNT(DISTINCT s.match_id, s.frame) FROM stalls s JOIN matches m USING (match_id) WHERE s.waited_ms >= 5000 AND " + IN_RANGE, 14, 0, 5, 4, "stat"),
    panel("Desynced games", "SELECT SUM(desync > 0) FROM matches m WHERE " + IN_RANGE, 19, 0, 5, 4, "stat"),
    panel("Games (click a game for detail)",
          "SELECT started_at AS Started, map AS Map, humans AS Players, router_name AS Router, version AS Version,"
          " ROUND(duration_s/60) AS Minutes, ROUND(sim_fps,1) AS `Speed fps`, slowest_name AS `Held speed down`, ROUND(felt_s) AS `Frozen s`,"
          " ROUND(felt_s*3600/duration_s) AS `Frozen s per hour`, ROUND(worst_stall_ms/1000,1) AS `Worst freeze s`,"
          " disc_screens AS `Disconnect screens`, desync AS Desync, CONCAT(reports,'/',humans) AS `Logs in`, match_id AS `Match`,"
          " UNIX_TIMESTAMP(started_at)*1000 - 60000 AS from_ms, UNIX_TIMESTAMP(ended_at)*1000 + 60000 AS to_ms"
          " FROM matches m WHERE " + IN_RANGE + " ORDER BY started_at DESC",
          0, 4, 24, 10, overrides=HIDE + [{"matcher": {"id": "byName", "options": "Match"},
                                             "properties": [{"id": "links", "value": [{"title": "Game detail", "url": MATCH_LINK}]}]}]),
    panel("Frozen seconds per hour of play, per game",
          "SELECT started_at AS time, felt_s*3600/duration_s AS `Frozen s per hour` FROM matches m WHERE " + IN_RANGE + " ORDER BY 1",
          0, 14, 12, 8, "timeseries", style="points", unit="s"),
    panel("Game speed per game (fps)",
          "SELECT started_at AS time, sim_fps AS `Speed fps` FROM matches m WHERE " + IN_RANGE + " ORDER BY 1",
          12, 14, 12, 8, "timeseries", style="points"),
    panel("Who the game waited on",
          "SELECT p.name AS Player, COUNT(*) AS Games, ROUND(SUM(p.lag_caused)) AS `Freeze s caused`,"
          " ROUND(AVG(p.lag_caused),1) AS `Per game` FROM match_players p JOIN matches m USING (match_id)"
          " WHERE p.lag_caused IS NOT NULL AND " + IN_RANGE + " GROUP BY p.name ORDER BY 3 DESC", 0, 22, 8, 10,
          desc=LAG_DESC),
    panel("By packet router",
          "SELECT router_name AS Router, COUNT(*) AS Games, ROUND(AVG(humans),1) AS `Avg players`,"
          " ROUND(SUM(frames)/SUM(duration_s),1) AS `Speed fps`, ROUND(SUM(felt_s)*3600/SUM(duration_s)) AS `Frozen s per hour`,"
          " ROUND(MAX(worst_stall_ms)/1000,1) AS `Worst freeze s` FROM matches m WHERE " + IN_RANGE + " GROUP BY router_name ORDER BY 2 DESC",
          8, 22, 8, 10),
    panel("By player count",
          "SELECT humans AS Players, COUNT(*) AS Games, ROUND(SUM(frames)/SUM(duration_s),1) AS `Speed fps`,"
          " ROUND(SUM(felt_s)*3600/SUM(duration_s)) AS `Frozen s per hour`, ROUND(MAX(worst_stall_ms)/1000,1) AS `Worst freeze s`"
          " FROM matches m WHERE " + IN_RANGE + " GROUP BY humans ORDER BY 1", 16, 22, 8, 10),
    panel("By release",
          "SELECT m.version AS Version, COUNT(*) AS Games, ROUND(SUM(frames)/SUM(duration_s),1) AS `Speed fps`,"
          " ROUND(SUM(felt_s)*3600/SUM(duration_s)) AS `Frozen s per hour`, SUM(desync > 0) AS Desyncs,"
          " SUM(humans) - SUM(reports) AS `Missing logs`, MIN(started_at) AS `First game`"
          " FROM matches m WHERE " + IN_RANGE + " GROUP BY m.version ORDER BY 7 DESC", 0, 32, 24, 7,
          desc="Missing logs = players in the roster whose log never arrived (crash, kill, or quit before upload)."),
    panel("Whose PC held the game speed down (2.1.6 and later clients only)",
          "SELECT p.name AS Player, COUNT(*) AS Games, ROUND(SUM(p.held_back * m.duration_s)/60, 1) AS `Minutes holding speed down`,"
          " ROUND(AVG(p.rep_fps)) AS `Avg PC fps`, ROUND(AVG(p.client_ms),1) AS `Client ms`, ROUND(AVG(p.logic_ms),1) AS `Logic ms`"
          " FROM match_players p JOIN matches m USING (match_id) WHERE p.rep_fps IS NOT NULL AND " + IN_RANGE +
          " GROUP BY p.name ORDER BY 3 DESC", 0, 39, 24, 9,
          desc="The router sets everyone's speed to the lowest PC fps. Client ms high = drawing is the limit (graphics settings, GPU);"
               " logic ms high = the CPU cannot keep up with the simulation."),
], desc="One row per game from the players' uploaded logs. Times are in your browser's time zone.")

# The profile's regulars: 3+ games of 2+ players in the last 30 days (lab accounts and one-off names drop out).
REG = ("(SELECT p2.name FROM match_players p2 JOIN matches m2 USING (match_id) WHERE p2.reported AND m2.humans >= 2"
       " AND m2.started_at >= NOW() - INTERVAL 30 DAY GROUP BY p2.name HAVING COUNT(*) >= 3)")
PAIR_DESC = ("One row per pair of players, both directions merged. 'Relay charged to' is whose fault the relays were, as the profile"
             " counts it: the player who needed relays with everyone else that game ('both' when it cannot tell, like a 1v1).")

M = "match_id = $match"
NEW = " (2.1.6 and later clients only)"
match = dashboard("zm-match", "Game detail", [
    panel("Game", "SELECT map AS Map, humans AS Players, router_name AS Router, version AS Version, ROUND(duration_s/60) AS Minutes,"
          " ROUND(sim_fps,1) AS `Speed fps`, slowest_name AS `Held speed down`, ROUND(felt_s) AS `Frozen s`,"
          " ROUND(worst_stall_ms/1000,1) AS `Worst freeze s`, disc_screens AS `Disconnect screens`, desync AS Desync"
          " FROM matches WHERE " + M, 0, 0, 24, 4),
    panel("Players", "SELECT slot AS Slot, name AS Player, IF(reported, IF(finished,'yes','no end'), 'no log') AS `Log`,"
          " version AS Version, ROUND(felt_stall_s,1) AS `Frozen s`, ROUND(lag_caused,1) AS `Freeze s caused`,"
          " ROUND(stall_s) AS `All waits s`, ROUND(stall_max_ms/1000,1) AS `Worst stall s`,"
          " disc_screens AS `Disconnect screens`, ROUND(rep_fps) AS `PC fps`, ROUND(held_back*100) AS `Held speed down %`,"
          " ROUND(client_ms,1) AS `Client ms`, ROUND(logic_ms,1) AS `Logic ms`, resend_req AS `Resends asked`,"
          " resend_ans AS `Resends served`, ping_mid_ms AS `Ping ms`, ROUND(direct_share*100) AS `Direct %`"
          " FROM match_players WHERE " + M + " ORDER BY slot", 0, 4, 24, 10,
          desc="Frozen s: waits past their first 100 ms (felt freezes); All waits s also counts the few ms waits that only slow the game."
               " Freeze s caused, Ping ms and Direct %: measured like the in-game profile (middle link, relays charged to the"
               " player at fault, freezes from the router's view). PC fps: what this PC reported to the router; the lowest sets the game speed. Held speed down: share of the game this PC was"
               " the slowest and under 54 fps. Client ms: input, UI and drawing per frame (includes any vsync wait). Logic ms: simulation per logic frame."),
    panel("Freezes as each player logged them, by who they were missing (raw)",
          "SELECT at AS time, blamed AS metric, waited_ms/1000 AS value FROM stalls WHERE " + M + " ORDER BY at",
          0, 14, 24, 8, "timeseries", style="points", unit="s",
          desc="Each 0.5 s+ freeze as a player's client logged it, named after whose moves that client was missing. A late relay"
               " shows up as the original sender here; 'Freeze s caused' in the Players table is the fair attribution."),
    panel("Game speed (frames per real second, 60 = full)",
          "SELECT at AS time, name AS metric, value FROM (SELECT at, name, (frame - LAG(frame) OVER w) / TIMESTAMPDIFF(SECOND, LAG(at) OVER w, at) AS value"
          " FROM netstat WHERE " + M + " WINDOW w AS (PARTITION BY slot ORDER BY at)) x WHERE value IS NOT NULL ORDER BY at",
          0, 22, 12, 8, "timeseries",
          desc="Logic frames each player advanced between NETSTAT lines. Dips are stalls; a flat line under 60 is the slowest PC setting the pace."),
    panel("Each PC's fps as the router saw it (lowest sets the speed)" + NEW,
          "SELECT at AS time, name AS metric, fps AS value FROM peer_fps WHERE " + M + " AND fps > 0 ORDER BY at",
          12, 22, 12, 8, "timeseries"),
    panel("Client time per frame: input, UI, drawing (ms)" + NEW,
          "SELECT at AS time, name AS metric, client_ms AS value FROM netstat WHERE " + M + " AND client_ms IS NOT NULL ORDER BY at",
          0, 30, 12, 8, "timeseries", unit="ms",
          desc="Average per drawn frame over 10 s. Above 16.7 ms a PC cannot draw 60 fps. Includes the vsync wait on a vsynced PC."),
    panel("Logic time per frame: the simulation (ms)" + NEW,
          "SELECT at AS time, name AS metric, logic_ms AS value FROM netstat WHERE " + M + " AND logic_ms IS NOT NULL ORDER BY at",
          12, 30, 12, 8, "timeseries", unit="ms",
          desc="Average per logic frame over 10 s. Every PC runs the same simulation, so a PC far above the rest is a slow CPU."),
    panel("Objects in the game" + NEW,
          "SELECT at AS time, 'objects' AS metric, objs AS value FROM netstat WHERE " + M + " AND objs IS NOT NULL"
          " AND slot = (SELECT router_slot FROM matches WHERE " + M + ") ORDER BY at",
          0, 38, 8, 8, "timeseries"),
    panel("Upload per player, all links (KB/s)" + NEW,
          "SELECT at AS time, name AS metric, SUM(out_kbps) AS value FROM routes WHERE " + M + " AND out_kbps >= 0 GROUP BY at, name ORDER BY at",
          8, 38, 16, 8, "timeseries", unit="KBs",
          desc="The packet router sends everyone's commands, so its upload is the one that can run out."),
    panel("Ping per link", "SELECT at AS time, CONCAT(name,' > ',peer) AS metric, ping_ms AS value FROM routes WHERE " + M + " AND ping_ms >= 0 ORDER BY at",
          0, 46, 12, 9, "timeseries", unit="ms"),
    panel("Send queue wait per link (ms)" + NEW,
          "SELECT at AS time, CONCAT(name,' > ',peer) AS metric, queue_ms AS value FROM routes WHERE " + M + " AND queue_ms >= 0 ORDER BY at",
          12, 46, 12, 9, "timeseries", unit="ms",
          desc="How long a new message waits before GNS sends it. Climbing on every link of one player at once = that player's upload is full."),
    panel("Unsent bytes queued per link", "SELECT at AS time, CONCAT(name,' > ',peer) AS metric, pending AS value FROM routes WHERE " + M + " AND pending >= 0 ORDER BY at",
          0, 55, 24, 8, "timeseries", unit="decbytes",
          desc="GNS pending reliable + unreliable bytes. A link that stops draining is about to time out (about 10 s)."),
    panel("Links", "SELECT l.a AS Player, l.b AS `With`, l.ping_ms AS `Ping ms`, ROUND(l.relayed*100) AS `Relayed %`,"
          " COALESCE(l.charged_to, '') AS `Relay charged to`, MAX(r.ping_ms) AS `Max ping ms`, MAX(r.pending) AS `Max queued bytes`,"
          " ROUND(AVG(r.out_kbps)) AS `Avg KB/s out`, MAX(r.queue_ms) AS `Max queue ms`"
          " FROM link_pairs l LEFT JOIN routes r ON r.match_id = l.match_id AND ((r.name = l.a AND r.peer = l.b) OR (r.name = l.b AND r.peer = l.a))"
          " WHERE l.match_id = $match GROUP BY l.a, l.b, l.ping_ms, l.relayed, l.charged_to ORDER BY l.relayed DESC, l.ping_ms DESC",
          0, 63, 24, 12, desc=PAIR_DESC + " Max ping, queued bytes, upload and queue time are the raw worst of the two directions."),
], time_from="now-24h", templating=[{
    "name": "match", "label": "Game", "type": "query", "datasource": DS, "refresh": 1,
    "query": "SELECT match_id AS __value, CONCAT(DATE_FORMAT(started_at,'%Y-%m-%d %H:%i'),' UTC  ',map,'  ',humans,'p') AS __text"
             " FROM matches ORDER BY started_at DESC LIMIT 300"}],
    desc="Open a game from the Game nights table so the time range matches it.")

FAIR = ("Measured like the in-game profile: ping is each game's middle link (one far away player does not count), direct %"
        " counts a relayed link only against the player who needed relays with everyone, and freeze caused comes from the"
        " packet router's own view (1v1 games and games without the router's log are left out).")


def form_col(key, label, value_sql):
    """One profile row as 'value · tier' from player_form's JSON."""
    return (" COALESCE(MAX(CASE WHEN x.k = '%s' THEN CONCAT(%s, ' · ', COALESCE(ELT(x.t + 1, 'Top tier', 'Solid', 'Rough patch'), 'Not yet')) END),"
            " 'Not enough games yet') AS `%s`" % (key, value_sql, label))


FORM_ROWS = " UNION ALL ".join(
    "SELECT f.name, x.* FROM player_form f, JSON_TABLE(f.form, '$.%s[*]' COLUMNS (k VARCHAR(16) PATH '$.k',"
    " v DOUBLE PATH '$.v', t INT PATH '$.t')) x" % sec for sec in ("net", "pc"))
BADGES = "JSON_UNQUOTE(JSON_EXTRACT(f.form, '$.badges'))"
for k, name in (("link", "Rock Solid Link"), ("nolag", "Never the Lag"), ("comms", "Frontline Comms"),
                ("machine", "War Machine"), ("improved", "Most Improved")):
    BADGES = "REPLACE(%s, '\"%s\"', '%s')" % (BADGES, k, name)
BADGES = "REPLACE(REPLACE(REPLACE(%s, '[', ''), ']', ''), '[]', '')" % BADGES

links = dashboard("zm-links", "Connections", [
    panel("Profile form (what players see in game, last 5 games)",
          "SELECT x.name AS Player, MAX(JSON_VALUE(f.form, '$.games')) AS Games," +
          form_col("ping", "Ping", "CONCAT(ROUND(x.v), ' ms')") + "," +
          form_col("direct", "Direct links", "CONCAT(ROUND(x.v * 100), '%')") + "," +
          form_col("lagcaused", "Lag caused", "CONCAT(ROUND(x.v, 1), ' s')") + "," +
          form_col("fps", "Game speed", "CONCAT(ROUND(x.v), ' fps')") + "," +
          form_col("heldback", "Slowest PC", "CONCAT(ROUND(x.v * 100), '%')") + "," +
          form_col("drawms", "Drawing", "CONCAT(ROUND(x.v, 1), ' ms')") + "," +
          form_col("simms", "Simulation", "CONCAT(ROUND(x.v, 1), ' ms')") + ","
          " MAX(" + BADGES + ") AS Badges"
          " FROM (" + FORM_ROWS + ") x JOIN player_form f ON f.name = x.name GROUP BY x.name ORDER BY x.name",
          0, 0, 24, 9,
          desc="The player_form table the GO Services hand to the Network Form screen, rebuilt every 5 minutes. Tiers compare with the"
               " regulars (3+ games in 30 days): Top tier = their best third, Rough patch = their worst third. " + FAIR),
    panel("Players", "SELECT p.name AS Player, COUNT(*) AS Games, ROUND(AVG(p.ping_mid_ms)) AS `Ping ms`,"
          " ROUND(AVG(p.direct_share)*100) AS `Direct %`, ROUND(AVG(p.lag_caused),1) AS `Freeze s caused per game`,"
          " ROUND(AVG(p.felt_stall_s),1) AS `Frozen s per game`, SUM(p.disc_screens) AS `Disconnect screens`,"
          " SUM(p.finished = 0) AS `Log without match end`, SUM(p.reported = 0) AS `No log`,"
          " ROUND(AVG(p.rep_fps)) AS `Avg PC fps`, ROUND(AVG(p.client_ms),1) AS `Client ms`, ROUND(AVG(p.logic_ms),1) AS `Logic ms`"
          " FROM match_players p JOIN matches m USING (match_id) WHERE p.name IN " + REG + " AND " + IN_RANGE +
          " GROUP BY p.name ORDER BY 2 DESC", 0, 9, 24, 10,
          desc="The regulars (3+ games in the last 30 days), averaged over every game in the time range (the profile uses the"
               " last 5). " + FAIR),
    panel("Links",
          "SELECT p.a AS Player, p.b AS `With`, p.games AS Games, p.ping AS `Ping ms`, p.rel AS `Relayed %`,"
          " COALESCE(c.charged, '') AS `Relay charged to (games)` FROM"
          " (SELECT l.a, l.b, COUNT(*) AS games, ROUND(AVG(l.ping_ms)) AS ping, ROUND(AVG(l.relayed)*100) AS rel"
          " FROM link_pairs l JOIN matches m USING (match_id) WHERE " + IN_RANGE + " AND l.a IN " + REG + " AND l.b IN " + REG +
          " GROUP BY l.a, l.b) p LEFT JOIN"
          " (SELECT a, b, GROUP_CONCAT(CONCAT(charged_to, ' (', n, ')') ORDER BY n DESC SEPARATOR ', ') AS charged FROM"
          " (SELECT l.a, l.b, l.charged_to, COUNT(*) AS n FROM link_pairs l JOIN matches m USING (match_id)"
          " WHERE " + IN_RANGE + " AND l.charged_to IS NOT NULL GROUP BY l.a, l.b, l.charged_to) z GROUP BY a, b) c"
          " ON c.a = p.a AND c.b = p.b ORDER BY p.rel DESC, p.ping DESC", 0, 19, 24, 10, desc=PAIR_DESC),
    panel("Freeze seconds caused, per game", "SELECT m.started_at AS time, p.name AS metric, p.lag_caused AS value FROM match_players p"
          " JOIN matches m USING (match_id) WHERE p.lag_caused IS NOT NULL AND " + IN_RANGE + " ORDER BY 1", 0, 29, 24, 8, "timeseries",
          style="points", unit="s", desc=LAG_DESC),
    panel("Ping per game (middle link)", "SELECT m.started_at AS time, p.name AS metric, p.ping_mid_ms AS value FROM match_players p"
          " JOIN matches m USING (match_id) WHERE p.ping_mid_ms IS NOT NULL AND " + IN_RANGE + " ORDER BY 1", 0, 37, 12, 8,
          "timeseries", style="points", unit="ms", desc=FAIR),
    panel("Direct links per game", "SELECT m.started_at AS time, p.name AS metric, p.direct_share*100 AS value FROM match_players p"
          " JOIN matches m USING (match_id) WHERE p.direct_share IS NOT NULL AND " + IN_RANGE + " ORDER BY 1", 12, 37, 12, 8,
          "timeseries", style="points", unit="percent", desc=FAIR),
], desc="Per player and per link connection quality across game nights.")

os.makedirs(OUT, exist_ok=True)
for d in (overview, match, links):
    with open(os.path.join(OUT, d["uid"] + ".json"), "w") as f:
        json.dump(d, f, indent=1)
print("wrote", OUT)
