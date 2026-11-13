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

PARSER = 3  # bump to re-ingest every bucket after a parser change
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
         "roster": {}, "humans": set(), "netstat": [], "stalls": [], "disc": [], "routes": [], "peers": []}
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
    out = ["DELETE FROM %s WHERE match_id=%d;\n" % (t, seed) for t in ("matches", "match_players", "stalls", "netstat", "routes", "peer_fps")]
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
                        round(sum(x[4] for x in rt if x[4] >= 0) / max(1, sum(1 for x in rt if x[4] >= 0))) if rt else None,
                        avg(fps_seen.get(r["slot"])), held.get(r["slot"]) if rr and rr["peers"] else None,
                        avg([n[13] for n in ns if n[13] is not None]), avg([n[15] for n in ns if n[15] is not None]),
                        max((n[17] for n in ns if n[17] is not None), default=None)))
        for at, t, frame, waited, on, blame in r["stalls"]:
            stalls.append((seed, name, r["slot"], ts(at), t, frame, waited, on, blame, blamed_name(blame, roster)))
        for at, t, frame, ra, fps, _router, lat, _rlat, sn, sms, smax, rreq, rans, cms, cmax, lms, lmax, objs in ns:
            netstat.append((seed, r["slot"], name, ts(at), t, frame, fps, ra, lat, sn, sms, smax, rreq, rans, cms, cmax, lms, lmax, objs))
        for at, t, peer, relayed, ping, pend, okb, qt in rt:
            routes.append((seed, r["slot"], name, peer, ts(at), t, relayed, ping, pend, okb, qt))
    reported_slots = {r["slot"] for r in reports.values()}
    for slot in sorted(humans - reported_slots):
        players.append((seed, slot, roster.get(slot), False) + (None,) * 15
                       + (avg(fps_seen.get(slot)), held.get(slot) if rr and rr["peers"] else None, None, None, None))

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
             roster.get(max(held, key=held.get)) if held and max(held.values()) > 0 else None)
    out.append(insert("matches", [match]))
    out.append(insert("match_players", players))
    out.append(insert("stalls", stalls))
    out.append(insert("netstat", netstat))
    out.append(insert("routes", routes))
    out.append(insert("peer_fps", peer_rows))
    out.append("REPLACE INTO ingest_state VALUES (%d,%d,%d,UTC_TIMESTAMP());\n" % (seed, newest, PARSER))
    return "START TRANSACTION;\n" + "".join(out) + "COMMIT;\n"


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
    elif sql:
        subprocess.run(DB_CMD, input=body, text=True, check=True)
    print("netmetrics: %d matches ingested, %d buckets without a match block" % (n_ok, n_skip))


if __name__ == "__main__":
    sys.exit(main())
