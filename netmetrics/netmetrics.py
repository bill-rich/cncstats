#!/usr/bin/env python3
"""Turn the per match client logs in the cncstats logs volume into rows in the
zulumetrics MariaDB database for Grafana.

Layout: <logs>/<seed>/<player>/ReleaseLog.txt[.gz] (+ GeneralsOnline.log[.gz]).
Each ReleaseLog is the whole client session; only the block between the
"Match start ... seed=<bucket>" line and the next "Match end" is used.

ReleaseLog lines carry a local HH:MM:SS clock only. The file's mtime is the
upload time, which is right after its last line, so a line's UTC time is
mtime minus (last line clock - line clock).

Usage:
  netmetrics.py [--logs DIR] [--since YYYY-MM-DD] [--sql-out FILE] [--all]
Without --sql-out the SQL is piped to the gosvc-db container.
"""
import argparse
import datetime as dt
import gzip
import json
import os
import re
import subprocess
import sys

PARSER = 6  # bump to re-ingest every bucket after a parser change
DB_CMD = ["docker", "exec", "-i", "gosvc-db", "sh", "-c",
          'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD" --default-character-set=utf8mb4']

CLOCK = re.compile(r"^\[(\d\d):(\d\d):(\d\d)\] (.*)$")
START = re.compile(r"=== Match start: map='([^']*)'.*?seed=(\d+).*?localSlot=(-?\d+)(?: resumeTo=(\d+))?")
END = re.compile(r"=== Match end: frame=(\d+)(?: desync=(\d+))?")
ROSTER = re.compile(r"ROSTER slot=(\d+) name=(.*?) (human|ai) template=")
NETSTAT = re.compile(r"NETSTAT frame=(\d+) ra=(-?\d+) fps=(\d+) router=(-?\d+) lat=(-?\d+)/(-?\d+)"
                     r".* stall=(\d+)/(\d+)ms/max(\d+).* rreq=(\d+)/(\d+)")
NETSTALL = re.compile(r"NETSTALL frame=(\d+) waited=(\d+)ms on=0x([0-9a-f]+)(?: router=0x([0-9a-f]+))?")
DISC_OFF = re.compile(r"DISCONNECT screen off frame=\d+ after (\d+)ms")
ROUTES = re.compile(r"GO routes: (.*)")
ROUTE = re.compile(r"(\d+):([DR])/(-?\d+)ms/q-?[\d.]+/pend(-?\d+)(?:/out(-?\d+)k/qt(-?\d+)ms)?")
# 2.1.6+: router's view of every PC's fps, and this PC's frame time split.
PEERS = re.compile(r"peers=\[([^\]]*)\]")
PEER = re.compile(r"(\d+):(-?\d+)/(-?\d+)")
STALL_ON = re.compile(r" on=\[([^\]]*)\]")
PERF = re.compile(r"client=(\d+\.\d)/max(\d+) logic=(\d+\.\d)/max(\d+) objs=(\d+)")
HELD_BACK_FPS = 54  # getMinimumFps ignores a PC within 10% of 60
VERSION = re.compile(r"CRASHREPORT enabled: (\S+)")
MEMBER = re.compile(r'"UserID":(\d+),"DisplayName":"((?:[^"\\]|\\.)*)"')


def read_text(path):
    for p, opener in ((path + ".gz", gzip.open), (path, open)):
        if os.path.exists(p):
            with opener(p, "rb") as f:
                return f.read().decode("utf-8", "replace"), os.path.getmtime(p)
    return None, None


def user_names(text):
    names = {}
    for uid, name in MEMBER.findall(text or ""):
        try:
            names[int(uid)] = json.loads('"%s"' % name)
        except ValueError:
            names[int(uid)] = name
    return names


def parse_report(text, mtime, seed, gonames):
    """One player's view of match <seed>, or None if the log lacks it."""
    lines = []
    for raw in text.splitlines():
        m = CLOCK.match(raw)
        if m:
            lines.append((int(m[1]) * 3600 + int(m[2]) * 60 + int(m[3]), m[4]))
    if not lines:
        return None
    last_clock = lines[-1][0]

    def utc(clock):
        return mtime - ((last_clock - clock) % 86400)

    # The last block for this seed (a resumed game logs a second start).
    start = None
    for i, (_, body) in enumerate(lines):
        m = START.search(body)
        if m and int(m[2]) == seed:
            start = i
    if start is None:
        return None
    m = START.search(lines[start][1])
    c0 = lines[start][0]
    r = {"map": os.path.splitext(re.split(r"[\\/]", m[1])[-1])[0], "slot": int(m[3]), "resume": int(m[4] or 0),
         "start": utc(c0), "end": None, "end_frame": None, "desync": None, "version": None,
         "roster": {}, "humans": set(), "netstat": [], "stalls": [], "disc": [], "routes": [], "peers": [],
         "windows": []}
    prev_at = r["start"]
    for _, body in lines[:start]:
        v = VERSION.search(body)
        if v:
            r["version"] = v[1].split("@")[-1]
    for clock, body in lines[start:]:
        t = (clock - c0) % 86400
        if (m := ROSTER.search(body)):
            r["roster"][int(m[1])] = m[2]
            if m[3] == "human":
                r["humans"].add(int(m[1]))
        elif (m := NETSTAT.search(body)):
            perf = PERF.search(body)
            perf = (float(perf[1]), int(perf[2]), float(perf[3]), int(perf[4]), int(perf[5])) if perf else (None,) * 5
            r["netstat"].append((utc(clock), t, *map(int, m.groups()), *perf))
            # One NETSTAT window: (from, to, felt stall ms, {slot: stalls waiting on it}).
            # The window holds only a total and a count, so the time past the first
            # NOTICE_MS of each wait is estimated as total - count * NOTICE_MS: never
            # more than the real figure. Waits of a few ms slow the game, they are not freezes.
            on = STALL_ON.search(body)
            r["windows"].append((prev_at, utc(clock), max(0, int(m[8]) - int(m[7]) * NOTICE_MS),
                                 {int(a): int(b) for a, b in (x.split(":") for x in on[1].split())} if on else {}))
            prev_at = utc(clock)
            if (pm := PEERS.search(body)):
                for slot, lat, fps in PEER.findall(pm[1]):
                    r["peers"].append((utc(clock), t, int(slot), int(lat), int(fps)))
        elif (m := NETSTALL.search(body)):
            on = int(m[3], 16)
            r["stalls"].append((utc(clock), t, int(m[1]), int(m[2]), on, int(m[4], 16) if m[4] else on))
        elif (m := DISC_OFF.search(body)):
            r["disc"].append(int(m[1]))
        elif (m := ROUTES.search(body)):
            for uid, kind, ping, pend, out, qt in ROUTE.findall(m[1]):
                peer = gonames.get(int(uid), "user %s" % uid)
                r["routes"].append((utc(clock), t, peer, kind == "R", int(ping), int(pend),
                                    int(out) if out else None, int(qt) if qt else None))
        elif (m := END.search(body)):
            r["end"], r["end_frame"] = utc(clock), int(m[1])
            r["desync"] = int(m[2]) if m[2] is not None else None
            break
    return r


def avg(xs):
    return sum(xs) / len(xs) if xs else None


def lag_caused(router, reports, slots):
    """Seconds of noticeable freeze (past NOTICE_MS per wait) each slot caused in one match, {slot: s}.
    Every PC's moves go through the router, so a non-router PC that is missing
    Neo's moves may be waiting on the router's relay, not on Neo. Only the router
    sees everyone directly, so its own stall windows decide who it waited on.
    Freezes the other PCs had and the router did not are relay or link delays:
    when most of them froze it is the router's, when one froze it is that PC's
    link. Without the router's log, or with only two players (nobody to tell
    whose end of the one link is bad), there is no fair answer: None."""
    rr = next((r for r in reports.values() if r["slot"] == router), None)
    if rr is None or len(slots) < 3:
        return None
    lag = dict.fromkeys(slots, 0.0)
    for _, _, ms, on in rr["windows"]:
        total = sum(c for s, c in on.items() if s != router)
        for s, c in on.items():
            if s != router and total:
                lag[s] = lag.get(s, 0.0) + ms / 1000.0 * c / total

    def router_ms(a, b):
        a, b = a - SKEW_S, b + SKEW_S  # each log's clock is rebuilt from its mtime
        return sum(ms * max(0.0, min(b, t1) - max(a, t0)) / max(1e-9, t1 - t0)
                   for t0, t1, ms, _ in rr["windows"] if t1 > a and t0 < b)
    others = [r for r in reports.values() if r["slot"] != router]
    buckets = {}
    for r in others:
        for t0, t1, ms, _ in r["windows"]:
            extra = ms - router_ms(t0, t1)
            if extra >= EXTRA_MIN_MS:
                buckets.setdefault(int(t1 // BUCKET_S), []).append((r["slot"], extra / 1000.0))
    for hits in buckets.values():
        if len({s for s, _ in hits}) * 2 >= len(others) and len(others) > 1:
            lag[router] = lag.get(router, 0.0) + sorted(x for _, x in hits)[len(hits) // 2]
        else:
            for s, x in hits:
                lag[s] = lag.get(s, 0.0) + x
    return lag


SKEW_S, BUCKET_S, EXTRA_MIN_MS, NOTICE_MS = 2, 10, 500, 100


def blamed_name(mask, roster):
    slots = [s for s in range(16) if mask & (1 << s)]
    if len(slots) == 1:
        return roster.get(slots[0], "slot %d" % slots[0])
    return "several" if slots else None


def q(v):
    if v is None:
        return "NULL"
    if isinstance(v, bool):
        return "1" if v else "0"
    if isinstance(v, (int, float)):
        return repr(round(v, 3)) if isinstance(v, float) else str(v)
    s = str(v)
    for a, b in (("\\", "\\\\"), ("'", "\\'"), ("\0", "\\0"), ("\n", "\\n"), ("\r", "\\r"), ("\x1a", "\\Z")):
        s = s.replace(a, b)
    return "'" + s + "'"


def ts(epoch):
    return dt.datetime.fromtimestamp(epoch, dt.timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


def insert(table, rows):
    if not rows:
        return ""
    vals = ",\n".join("(" + ",".join(q(v) for v in row) + ")" for row in rows)
    return "INSERT INTO %s VALUES\n%s;\n" % (table, vals)


def bucket_sql(seed, reports, newest):
    """SQL replacing everything for one match. reports: {uploader: report}."""
    out = ["DELETE FROM %s WHERE match_id=%d;\n" % (t, seed) for t in ("matches", "match_players", "stalls", "netstat", "routes", "peer_fps", "link_pairs")]
    roster, humans = {}, set()
    for r in reports.values():
        roster.update(r["roster"])
        humans |= r["humans"]
    router = next((n[5] for r in reports.values() for n in r["netstat"] if n[5] >= 0), None)

    # The router's fps reports: the lowest one sets everyone's game speed.
    peer_rows, fps_seen, held = [], {}, {}
    rr = next((r for r in reports.values() if r["slot"] == router), None)
    if rr:
        by_sample = {}
        for at, t, slot, lat, fps in rr["peers"]:
            peer_rows.append((seed, slot, roster.get(slot), ts(at), t, lat, fps))
            if fps > 0:
                by_sample.setdefault(at, []).append((fps, slot))
                fps_seen.setdefault(slot, []).append(fps)
        for sample in by_sample.values():
            low = min(sample)[0]
            for fps, slot in sample:
                held[slot] = held.get(slot, 0) + (fps == low and low < HELD_BACK_FPS)
        held = {slot: n / len(by_sample) for slot, n in held.items()} if by_sample else {}

    lag = lag_caused(router, reports, humans | {r["slot"] for r in reports.values()})
    # Fair link stats per player (link_metrics), stored so the dashboards match the profile.
    dirs = {}
    for name, r in reports.items():
        for _, _, peer, relayed, ping, *_ in r["routes"]:
            d = dirs.setdefault((name, peer), ([], []))
            d[0].append(relayed)
            if ping > 0:
                d[1].append(ping)
    pairs = []
    fair = link_metrics([(seed, a, b, avg(rel), avg(pings) if pings else None) for (a, b), (rel, pings) in dirs.items()], pairs)

    def fair_cols(name):
        ping, direct = fair.get((seed, name), (None, None))
        return (round(ping) if ping is not None else None, direct)
    players, stalls, netstat, routes = [], [], [], []
    for name, r in sorted(reports.items()):
        ns = r["netstat"]
        rt = r["routes"]
        players.append((seed, r["slot"], name, True, r["version"], r["end"] is not None, r["end_frame"],
                        sum(n[8] for n in ns), sum(n[9] for n in ns) / 1000.0, max((n[10] for n in ns), default=0),
                        len(r["disc"]), sum(r["disc"]) / 1000.0,
                        round(sum(n[6] for n in ns) / len(ns)) if ns else None,
                        min((n[4] for n in ns if n[4] > 0), default=None),
                        sum(n[11] for n in ns), sum(n[12] for n in ns),
                        len(rt), (sum(1 for x in rt if x[3]) / len(rt)) if rt else None,
                        round(avg([x[4] for x in rt if x[4] >= 0])) if any(x[4] >= 0 for x in rt) else None,
                        avg(fps_seen.get(r["slot"])), held.get(r["slot"]) if rr and rr["peers"] else None,
                        avg([n[13] for n in ns if n[13] is not None]), avg([n[15] for n in ns if n[15] is not None]),
                        max((n[17] for n in ns if n[17] is not None), default=None),
                        lag.get(r["slot"]) if lag is not None else None,
                        sum(w[2] for w in r["windows"]) / 1000.0, *fair_cols(name)))
        for at, t, frame, waited, on, blame in r["stalls"]:
            stalls.append((seed, name, r["slot"], ts(at), t, frame, waited, on, blame, blamed_name(blame, roster)))
        for at, t, frame, ra, fps, _router, lat, _rlat, sn, sms, smax, rreq, rans, cms, cmax, lms, lmax, objs in ns:
            netstat.append((seed, r["slot"], name, ts(at), t, frame, fps, ra, lat, sn, sms, smax, rreq, rans, cms, cmax, lms, lmax, objs))
        for at, t, peer, relayed, ping, pend, okb, qt in rt:
            routes.append((seed, r["slot"], name, peer, ts(at), t, relayed, ping, pend, okb, qt))
    reported_slots = {r["slot"] for r in reports.values()}
    for slot in sorted(humans - reported_slots):
        players.append((seed, slot, roster.get(slot), False) + (None,) * 15
                       + (avg(fps_seen.get(slot)), held.get(slot) if rr and rr["peers"] else None, None, None, None,
                          lag.get(slot) if lag is not None else None, None, *fair_cols(roster.get(slot))))

    # Match timing from the reporter who got furthest (early leavers stop short).
    best = max(reports.values(), key=lambda r: (r["end_frame"] or 0, r["end"] or 0))
    start = min(r["start"] for r in reports.values())
    end = best["end"] or max((n[0] for n in best["netstat"]), default=best["start"])
    duration = max(1, round(end - best["start"]))
    frames = best["end_frame"] or max((n[2] for n in best["netstat"]), default=0)
    versions = sorted({r["version"] for r in reports.values() if r["version"]})
    match = (seed, ts(start), ts(end), best["map"], len(humans) or None, router,
             roster.get(router) if router is not None else None, ",".join(versions) or None,
             duration, frames, (frames - best["resume"]) / duration,
             max((r["desync"] or 0 for r in reports.values()), default=0), len(reports),
             max((p[8] or 0 for p in players), default=0), max((p[9] or 0 for p in players), default=0),
             max((p[10] or 0 for p in players), default=0),
             roster.get(max(held, key=held.get)) if held and max(held.values()) > 0 else None,
             max((p[-1] or 0 for p in players), default=0))
    out.append(insert("matches", [match]))
    out.append(insert("match_players", players))
    out.append(insert("stalls", stalls))
    out.append(insert("netstat", netstat))
    out.append(insert("routes", routes))
    out.append(insert("peer_fps", peer_rows))
    out.append(insert("link_pairs", pairs))
    out.append("REPLACE INTO ingest_state VALUES (%d,%d,%d,UTC_TIMESTAMP());\n" % (seed, newest, PARSER))
    return "START TRANSACTION;\n" + "".join(out) + "COMMIT;\n"


# Player profile "form": each player's last 5 games on each metric, as a tier
# against the regulars, for the in-game profile (Services passes it through).
# Only stats that point at the player who caused them: link stats are charged to
# the end that is bad with everyone (link_metrics), lag caused comes from the
# router's own view (lag_caused), and the PC stats are measured per PC.
# (key, section, higher_is_better, noise floor for the trend)
METRICS = [("ping", "net", False, 5), ("direct", "net", True, 0.05), ("lagcaused", "net", False, 0.5),
           ("fps", "pc", True, 2), ("heldback", "pc", False, 0.02), ("drawms", "pc", False, 1.0), ("simms", "pc", False, 0.5)]
FORM_GAMES, FORM_MIN, REGULAR_GAMES, REGULAR_DAYS, TIER_MIN_PLAYERS = 5, 3, 3, 30, 4
NOLAG_S = 2.0  # "Never the Lag": an occasional hiccup is fine
FORM_SQL = ("SELECT p.match_id, p.name, UNIX_TIMESTAMP(m.started_at), p.ping_mid_ms, p.direct_share, p.lag_caused,"
            " p.rep_fps, p.held_back, p.client_ms, p.logic_ms"
            " FROM zulumetrics.match_players p JOIN zulumetrics.matches m USING (match_id)"
            " WHERE p.reported = 1 AND p.name IS NOT NULL AND m.humans >= 2 ORDER BY m.started_at")



def link_metrics(link_rows, pairs_out=None):
    """link_rows: (match, reporter, peer, relayed share, avg ping or None), one per direction.
    Returns {(match, name): (ping, direct)}: ping is the player's middle link, so one far
    away peer does not move it; a relayed link counts against the end with more relayed
    links to everyone else (half each on a tie), so one player stuck behind a relay is
    the only one charged for it. pairs_out, when given, gets one row per pair:
    (match, a, b, relayed share, ping, charged_to) with charged_to a, b, 'both' or None."""
    links = {}
    for match, a, b, relayed, ping in link_rows:
        if a != b:
            links.setdefault((match, frozenset((a, b))), []).append((relayed, ping))
    per_match = {}
    for (match, pair), sides in links.items():
        pings = [p for _, p in sides if p is not None]
        share = avg([r for r, _ in sides if r is not None]) or 0
        per_match.setdefault(match, {})[pair] = (share >= 0.5, avg(pings) if pings else None, share)
    out = {}
    for match, ls in per_match.items():
        mine = {}
        for pair in ls:
            for x in pair:
                mine.setdefault(x, []).append(pair)

        def bad_elsewhere(x, pair):
            other = [l for l in mine[x] if l != pair]
            return sum(ls[l][0] for l in other) / len(other) if other else 0
        fault = dict.fromkeys(mine, 0.0)
        for pair, (relayed, ping, share) in ls.items():
            a, b = sorted(pair)
            charged = None
            if relayed:
                fa, fb = bad_elsewhere(a, pair), bad_elsewhere(b, pair)
                fault[a] += 1 if fa > fb else 0.5 if fa == fb else 0
                fault[b] += 1 if fb > fa else 0.5 if fa == fb else 0
                charged = a if fa > fb else b if fb > fa else "both"
            if pairs_out is not None:
                pairs_out.append((match, a, b, round(share, 3), round(ping) if ping is not None else None, charged))
        for x, pairs in mine.items():
            pings = sorted(ls[l][1] for l in pairs if ls[l][1] is not None)
            mid = (pings[(len(pings) - 1) // 2] + pings[len(pings) // 2]) / 2 if pings else None
            out[(match, x)] = (mid, 1 - fault[x] / len(pairs))
    return out


def compute_forms(rows, now):
    """rows: (name, started_epoch, *one value per METRICS entry, None = not measured), oldest first.
    Returns {name: form dict}. Tier 0 = top third of the regulars, 1 = middle, 2 = bottom third,
    -1 = too few games or too few regulars to say."""
    games = {}
    for name, at, *vals in rows:
        games.setdefault(name, []).append((at, vals))
    regulars = {n for n, g in games.items() if sum(1 for at, _ in g if at >= now - REGULAR_DAYS * 86400) >= REGULAR_GAMES}
    forms = {n: {"games": min(len(g), FORM_GAMES), "net": [], "pc": [], "badges": []} for n, g in games.items()}
    stats = {}
    for i, (key, section, up, floor) in enumerate(METRICS):
        better = (lambda a, b: a >= b) if up else (lambda a, b: a <= b)
        per = {}
        for n, g in games.items():
            vals = [v[i] for _, v in g if v[i] is not None]
            if len(vals) >= FORM_MIN:
                last, prev = vals[-FORM_GAMES:], vals[-2 * FORM_GAMES:-FORM_GAMES]
                per[n] = (avg(last), last, avg(prev) if len(prev) >= FORM_MIN else None)
        ranked = sorted((per[n][0] for n in per if n in regulars), reverse=up)
        k = len(ranked) // 3
        cuts = (ranked[k - 1], ranked[len(ranked) - k]) if len(ranked) >= TIER_MIN_PLAYERS and k else None

        def tier(v):
            if cuts is None:
                return -1
            return 0 if better(v, cuts[0]) else 2 if better(cuts[1], v) else 1
        stats[key] = per
        for n, (mean, last, prev) in per.items():
            trend = None
            if prev is not None:
                delta = (mean - prev) if up else (prev - mean)
                trend = 1 if delta > max(abs(prev) * 0.15, floor) else -1 if -delta > max(abs(prev) * 0.15, floor) else 0
            forms[n][section].append({"k": key, "v": round(mean, 3), "p": round(prev, 3) if prev is not None else None, "t": tier(mean),
                                      "d": "".join("x" if tier(v) < 0 else str(tier(v)) for v in last), "tr": trend})

    def best(key, up):
        cand = [(v[0], n) for n, v in stats[key].items() if n in regulars]
        return (max if up else min)(cand)[1] if len(cand) >= TIER_MIN_PLAYERS else None
    for n, g in games.items():
        last = g[-FORM_GAMES:]
        if len(last) == FORM_GAMES and all(v[1] is not None and v[1] >= 1 for _, v in last):
            forms[n]["badges"].append("link")       # never the reason for a relay in 5 games
        lag = stats["lagcaused"].get(n)
        if lag and lag[0] < NOLAG_S:
            forms[n]["badges"].append("nolag")      # under NOLAG_S of felt freeze per game
    for key, up, badge in (("ping", False, "comms"), ("fps", True, "machine")):
        if (n := best(key, up)):
            forms[n]["badges"].append(badge)
    drops = [((prev - mean) / prev, n) for n, (mean, _, prev) in stats["lagcaused"].items()
             if n in regulars and prev and prev >= 1]
    if drops and max(drops)[0] >= 0.25:
        forms[max(drops)[1]]["badges"].append("improved")
    return forms


def form_rows(form_tsv):
    """FORM_SQL's tab separated output -> compute_forms rows."""
    def num(x):
        return None if x == "NULL" else float(x)
    return [(f[1], int(f[2]), *(num(x) for x in f[3:])) for f in (l.split("\t") for l in form_tsv.splitlines() if l)]


def add_honors(forms, rows):
    """Copy honors.py's daily honors (name, JSON list) into the forms, by name, ignoring case.
    A holder without a form (no Zulu GO game logged) gets one with only the honors."""
    by_lower = {n.lower(): n for n in forms}
    for name, text in rows:
        try:
            honors = json.loads(text)
        except ValueError:
            continue
        n = by_lower.get(name.lower())
        if n is None:
            n = by_lower[name.lower()] = name
            forms[n] = {"games": 0, "net": [], "pc": [], "badges": []}
        forms[n]["honors"] = honors


def form_sql(forms):
    rows = [(n, json.dumps(f, separators=(",", ":"))) for n, f in sorted(forms.items())]
    body = "".join("(%s,%s,UTC_TIMESTAMP())" % (q(n), q(f)) + (",\n" if i < len(rows) - 1 else ";\n")
                   for i, (n, f) in enumerate(rows))
    return "START TRANSACTION;\nDELETE FROM player_form;\n" + ("INSERT INTO player_form VALUES\n" + body if rows else "") + "COMMIT;\n"


def newest_mtime(path):
    return int(max((os.path.getmtime(os.path.join(d, f)) for d, _, fs in os.walk(path) for f in fs), default=0))


def ingest_state():
    try:
        out = subprocess.run(DB_CMD[:-1] + [DB_CMD[-1] + " -N -e 'SELECT bucket, newest_mtime, parser FROM zulumetrics.ingest_state'"],
                             check=True, capture_output=True, text=True).stdout
    except subprocess.CalledProcessError:
        return {}
    return {int(b): (int(m), int(p)) for b, m, p in (l.split("\t") for l in out.splitlines() if l)}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--logs", default="/var/lib/docker/volumes/cncstats_logs_data/_data")
    ap.add_argument("--since", default="2026-10-20", help="skip buckets last written before this date (UTC)")
    ap.add_argument("--sql-out", help="write SQL here instead of loading it")
    ap.add_argument("--all", action="store_true", help="ignore ingest_state and redo every bucket")
    a = ap.parse_args()

    since = dt.datetime.strptime(a.since, "%Y-%m-%d").replace(tzinfo=dt.timezone.utc).timestamp()
    done = {} if (a.all or a.sql_out) else ingest_state()
    sql, n_ok, n_skip = [], 0, 0
    for bucket in sorted(os.listdir(a.logs)):
        if not bucket.isdigit():
            continue  # crash-<date>, connfail-<date>
        path = os.path.join(a.logs, bucket)
        newest = newest_mtime(path)
        seed = int(bucket)
        if newest < since or done.get(seed) == (newest, PARSER):
            continue
        reports = {}
        for player in sorted(os.listdir(path)):
            pdir = os.path.join(path, player)
            text, mtime = read_text(os.path.join(pdir, "ReleaseLog.txt"))
            if text is None:
                continue
            gonames = user_names(read_text(os.path.join(pdir, "GeneralsOnline.log"))[0])
            r = parse_report(text, mtime, seed, gonames)
            if r:
                reports[player] = r
        if not reports:
            n_skip += 1
            continue
        sql.append(bucket_sql(seed, reports, newest))
        n_ok += 1
    body = "SET NAMES utf8mb4;\nUSE zulumetrics;\n" + "".join(sql)
    if a.sql_out:
        with open(a.sql_out, "w") as f:
            f.write(body)
        return
    if sql:
        subprocess.run(DB_CMD, input=body, text=True, check=True)
    print("netmetrics: %d matches ingested, %d buckets without a match block" % (n_ok, n_skip))
    # Rebuilt every run: the regulars window moves with the clock even when no game arrived.
    out = subprocess.run(DB_CMD[:-1] + [DB_CMD[-1] + ' -N -e "%s"' % FORM_SQL], check=True, capture_output=True, text=True).stdout
    rows = form_rows(out)
    forms = compute_forms(rows, dt.datetime.now(dt.timezone.utc).timestamp())
    honors = subprocess.run(DB_CMD[:-1] + [DB_CMD[-1] + ' -N -e "SELECT name, honors FROM zulumetrics.player_honors"'],
                            capture_output=True, text=True).stdout
    add_honors(forms, [l.split("\t", 1) for l in honors.splitlines() if "\t" in l])
    subprocess.run(DB_CMD, input="SET NAMES utf8mb4;\nUSE zulumetrics;\n" + form_sql(forms), text=True, check=True)
    print("netmetrics: form rebuilt for %d players" % len(forms))


if __name__ == "__main__":
    sys.exit(main())
