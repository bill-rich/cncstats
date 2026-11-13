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
assert ",41.0,1.0,NULL,NULL,NULL)" in sql, sql  # O'Neil held the game back
assert ",60.0,0.0,3.5,1.2,812)" in sql and ",'O\\'Neil')" in sql, sql
print("ok")
