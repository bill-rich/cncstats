#!/usr/bin/env python3
"""python3 -I netmetrics/test_netmetrics.py"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import netmetrics as nm

LOG = """[23:58:00] CRASHREPORT enabled: zulugo@2.1.5 -> https://x/crash/1
[23:59:50] === Match start: map='c:\\maps\\a b\\a b.map' mode=5 seed=111 logicFps=60 localSlot=1 resumeTo=600 ===
[23:59:50] ROSTER slot=0 name=Skip human template=2 color=0 start=0 team=-1 ip=0.0.0.0:0
[23:59:50] ROSTER slot=1 name=O'Neil human template=2 color=0 start=0 team=-1 ip=0.0.0.0:0
[00:00:05] NETSTAT frame=600 ra=16 fps=60 router=0 lat=0/50 links=[0:0+0/40] stall=2/700ms/max600 on=[0:2] hitch=0/max0 rreq=1/1 peers=[]
[00:00:06] NETSTALL frame=610 waited=600ms on=0x01
[00:00:07] GO routes: 2:R/40ms/q1.00/pend3
[00:00:10] === Match end: frame=1200 desync=0 ===
[00:00:20] Log upload: 1 bytes -> 200
"""

r = nm.parse_report(LOG, 1000000.0, 111, {2: "Skip"})
assert r["resume"] == 600 and r["map"] == "a b" and r["slot"] == 1 and r["version"] == "2.1.5", r
assert r["start"] == 1000000 - 30 and r["end"] == 1000000 - 10, (r["start"], r["end"])  # crosses midnight
assert r["netstat"][0][1] == 15 and r["stalls"][0][5] == 1
assert r["routes"] == [(1000000 - 13, 17, "Skip", True, 40, 3, None, None)]
assert nm.parse_report(LOG, 0, 222, {}) is None
assert nm.blamed_name(1, {0: "Skip"}) == "Skip" and nm.blamed_name(3, {}) == "several"
assert nm.q("a'\\b") == "'a\\'\\\\b'" and nm.q(None) == "NULL"
sql = nm.bucket_sql(111, {"O'Neil": r}, 5)
assert ",30.0," in sql and "'O\\'Neil'" in sql and sql.count("INSERT INTO") == 5, sql
# 2.1.6 router: per PC fps in peers=, frame time split, upload per link.
ROUTER = LOG.replace("localSlot=1", "localSlot=0").replace(
    "rreq=1/1 peers=[]", "rreq=1/1 peers=[0:10/60 1:50/41] client=3.5/max12 logic=1.2/max4 objs=812").replace(
    "pend3", "pend3/out24k/qt35ms")
rr = nm.parse_report(ROUTER, 1000000.0, 111, {2: "Skip"})
assert rr["peers"] == [(1000000 - 15, 15, 0, 10, 60), (1000000 - 15, 15, 1, 50, 41)], rr["peers"]
assert rr["netstat"][0][13:] == (3.5, 12, 1.2, 4, 812) and r["netstat"][0][13:] == (None,) * 5
assert rr["routes"][0][6:] == (24, 35) and r["routes"][0][6:] == (None, None)
sql = nm.bucket_sql(111, {"O'Neil": r, "Skip": rr}, 5)
assert "INSERT INTO peer_fps" in sql and sql.rstrip().endswith("COMMIT;")
assert ",41.0,1.0,NULL,NULL,NULL,NULL,0.5)" in sql, sql  # O'Neil held the game back; 1v1: lag unmeasured
assert ",60.0,0.0,3.5,1.2,812,NULL,0.5)" in sql and ",'O\\'Neil',0.5)" in sql, sql
# Lag caused, router slot 0 with three others. Window A (0-10 s): the router
# waited 2 s on slot 3, everyone froze with it. Window B (100-110 s): the router
# was fine but slots 1, 2 and 3 froze 3 s: the relay. Window C (200-210 s): only
# slot 2 froze, 1 s: its own link.
def rep_(slot, wins):
    return {"slot": slot, "windows": wins}
R = {"r": rep_(0, [(0, 10, 2000, {3: 1}), (100, 110, 0, {}), (200, 210, 0, {})]),
     "a": rep_(1, [(0, 10, 2000, {3: 1}), (100, 110, 3000, {2: 1}), (200, 210, 0, {})]),
     "b": rep_(2, [(0, 10, 2000, {3: 1}), (100, 110, 3000, {3: 1}), (200, 210, 1000, {0: 1})]),
     "c": rep_(3, [(0, 10, 0, {}), (100, 110, 3000, {1: 1}), (200, 210, 0, {})])}
lag = nm.lag_caused(0, R, {0, 1, 2, 3})
w = nm.parse_report(LOG, 1000000.0, 111, {})["windows"]
assert w == [(1000000 - 30, 1000000 - 15, 500, {0: 2})], w  # 2 stalls, 700 ms: 500 ms past the first 100 ms each
assert lag == {0: 3.0, 1: 0.0, 2: 1.0, 3: 2.0}, lag
assert nm.lag_caused(5, R, {0}) is None  # no router log: no fair answer
assert nm.lag_caused(0, {"r": R["r"], "a": R["a"]}, {0, 1}) is None  # 1v1: cannot tell whose end

# Link attribution: S is relayed with everyone, A-B-C are direct with each other,
# and C is far away from everybody. Only S pays for the relays, only C for the ping.
L = []
for x, y in (("A", "B"), ("A", "C"), ("B", "C"), ("A", "S"), ("B", "S"), ("C", "S")):
    far = 200 if "C" in (x, y) else 30
    for a_, b_ in ((x, y), (y, x)):
        L.append((7, a_, b_, 1.0 if "S" in (x, y) else 0.0, far))
lm = nm.link_metrics(L)
assert lm[(7, "S")][1] == 0 and lm[(7, "A")][1] == 1 and lm[(7, "C")][1] == 1, lm
assert lm[(7, "A")][0] == 30 and lm[(7, "C")][0] == 200, lm
assert nm.link_metrics([(1, "A", "B", 1.0, None), (1, "B", "A", 1.0, None)])[(1, "A")] == (None, 0.5)  # 1v1: split
rows = nm.form_rows("7\tA\t100\t0\t60\t0\t9.5\t3\n", "7\tA\tB\t0.0000\t40.0\n7\tB\tA\tNULL\tNULL\n")
assert rows == [("A", 100, 40.0, 1.0, 0.0, 60.0, 0.0, 9.5, 3.0)], rows

# Profile form: 6 regulars, ping 10..60 ms (lower is better), 6 games each.
NOW = 10 * 86400 * 365
rows = []
for g in range(6):
    for i, n in enumerate("ABCDEF"):
        lag = (4.0 if g < 1 else 1.0) if n == "B" else 2.0 if n == "F" else 0
        rows.append((n, NOW - (6 - g) * 3600, 10 + 10 * i, 1.0 if n == "A" else 0.5, lag, 60 - i, 0.0, 9.0, 3.0))
rows.append(("Old", NOW - 90 * 86400, 5, 1.0, 0, 60, 0, 1, 1))  # not a regular: one game long ago
f = nm.compute_forms(rows, NOW)
ping = {n: next(m for m in f[n]["net"] if m["k"] == "ping") for n in "ABCDEF"}
assert [ping[n]["t"] for n in "ABCDEF"] == [0, 0, 1, 1, 2, 2], ping
assert ping["A"]["v"] == 10 and ping["A"]["d"] == "00000" and f["A"]["games"] == 5
assert "Old" in f and f["Old"]["net"] == []          # 1 game: under FORM_MIN, no rows at all
assert [m["k"] for m in f["A"]["net"]] == ["ping", "direct", "lagcaused"] and len(f["A"]["pc"]) == 4
assert "link" in f["A"]["badges"] and "link" not in f["B"]["badges"]
assert "nolag" in f["A"]["badges"] and "nolag" not in f["F"]["badges"]
assert "comms" in f["A"]["badges"] and "machine" in f["A"]["badges"]
assert next(m for m in f["B"]["net"] if m["k"] == "lagcaused")["tr"] is None  # only 1 earlier game
sql = nm.form_sql({"O'Neil": f["A"]})
assert sql.startswith("START TRANSACTION;\nDELETE FROM player_form;") and "'O\\'Neil'" in sql, sql
print("ok")
