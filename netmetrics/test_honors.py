#!/usr/bin/env python3
"""python3 -I netmetrics/test_honors.py"""
import os
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import honors as h

h.MIN_GAMES = 2


def game(i, cat, dur, players, details=None):
    return {"id": i, "timestamp": "2026-11-%02dT00:00:00Z" % (i + 1), "duration_minutes": dur, "map": "maps/[rank] alpha zh",
            "composition": {"category": cat, "is1V1": cat == "1v1"}, "details": details,
            "players": [{"name": n, "general": g, "won": w} for n, g, w in players]}


kills = lambda killer, unit, n, value=100: {"player_summary": [{"Name": "Alice"}, {"Name": "Bob"}],
                                           "killEvents": [{"killerPlayer": killer, "victimPlayer": "x", "killer": unit, "value": value}] * n}
games = [game(i, "1v1", 10, [("Alice", 8, True), ("Bob", 0, False)], kills("Alice", "Demo_GLAInfantryTerrorist", 3)) for i in range(4)]
games += [game(4, "1v1", 30, [("Alice", 8, False), ("Bob", 0, True)], kills("Bob", "AmericaFireBase", 9))]
games += [game(5, "1v1", 4, [("Alice", 8, True), ("Bob", 0, False)])]
out = h.compute(games, {})
keys = {n: {x["k"]: x["why"] for x in v} for n, v in out.items()}
assert "streak" not in keys["Alice"] and "streak" not in keys.get("Bob", {})  # needs 3 in a row: Alice has 1
assert keys["Alice"]["terrorist"] == "12 kills, $1,200 destroyed", keys
assert keys["Bob"]["holdline"] == "9 kills by base defenses" and keys["Bob"]["ironwill"] == "1 long game wins", keys
assert keys["Alice"]["general"] == "GLA, 83% of 6 games" and keys["Alice"]["duelist"] == "won 5 of 6 1v1s", keys
assert sum(len(v) for v in out.values()) == len({x["k"] for v in out.values() for x in v})  # one holder per honor

# champions: the 1v1 champion is remembered across a new, unfinished bracket; 2v2 from the newest complete one
with tempfile.TemporaryDirectory() as d:
    state = os.path.join(d, "champions.json")
    tourns = [{"slug": "t2", "format": "2v2_round_robin", "status": "complete"}]
    res = [{"tournament": {"name": "t2", "teams": [["A", "B"], ["C", "D"]]}, "matchups": [{"matches": [
        {"players": [{"name": "A", "team": 1, "won": True}, {"name": "B", "team": 1, "won": True},
                     {"name": "C", "team": 2, "won": False}, {"name": "D", "team": 2, "won": False}]}]}]}]
    c = h.champions({"champion": "Modus", "runner_up": "Gorn"}, tourns, res, state)
    assert c["Modus"] == [("champ1v1", "beat Gorn in the final")] and c["A"] == [("champ2v2", "with B, 1-0")], c
    assert h.champions({"champion": None}, [], [], state)["Modus"][0][0] == "champ1v1"
assert h.canon("wld") == "WildCard" and h.unit("Demo_GLAInfantryTerrorist") == "GLAInfantryTerrorist"
assert h.honors_sql({"O'Neil": [{"k": "map", "why": "x"}]}).count("INSERT INTO player_honors") == 1
print("ok")
