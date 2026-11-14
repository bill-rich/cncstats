#!/usr/bin/env python3
"""Daily: single holder honors (in-game profile badges) from radarvan stats.

Reads the last WINDOW_DAYS of radarvan games (game lists and per game details,
cached under CACHE; finished days and details never change, so a run only
fetches what is new), the 1v1 bracket and the 2v2 tournament results, picks
one holder per honor among the regulars (MIN_GAMES+ games in the window), and
replaces zulumetrics.player_honors. netmetrics.py adds each player's honors
to their player_form, which the GO Services hand to the game.

Usage: honors.py [--cache DIR] [--dry-run]
Needs RADARVAN_KEY (env, or /opt/netmetrics/radarvan.env).
"""
import argparse
import collections as C
import datetime as dt
import json
import os
import statistics as st
import sys
import time
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from netmetrics import DB_CMD, q  # noqa: E402
import subprocess  # noqa: E402

BASE = "https://www.radarvan.com"
WINDOW_DAYS, MIN_GAMES, REFETCH_DAYS = 90, 10, 2
GEN = {0: "USA", 1: "Air Force", 2: "Laser", 3: "Superweapon", 4: "China", 5: "Nuke", 6: "Tank", 7: "Infantry",
       8: "GLA", 9: "Toxin", 10: "Stealth", 11: "Demolition"}
HEROES = ("ColonelBurton", "BlackLotus", "JarmenKell")
SUPERWEAPONS = ("ParticleUplinkCannon", "NeutronMissile", "ScudStorm")
RARE = ("AmericaTankMicrowave", "GLAInfantrySaboteur", "GLATankMarauder", "AmericaTankPaladin",
        "GLAVehicleBombTruck", "AmericaJetStealthFighter")
DEFENSES = ("AmericaFireBase", "AmericaPatriotBattery", "AmericaStrategyCenter", "ChinaGattlingCannon",
            "ChinaBunker", "GLAStingerSite", "GLATunnelNetwork", "GLADemoTrap")
ALIASES = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "radarvan_aliases.json")))


def canon(name):
    """radarvan's raw in-game name -> its canonical player (radarvan's PLAYER_NAME_MAPPING)."""
    return ALIASES.get(name.lower(), name)


def unit(template):
    """Drop the general prefix: Demo_GLAInfantryTerrorist -> GLAInfantryTerrorist."""
    return template.split("_", 1)[1] if "_" in template else template


class Radarvan:
    """Serial, polite fetches: the radarvan dyno fell over under parallel detail pulls once."""

    def __init__(self, key, cache):
        self.key, self.cache, self.calls = key, cache, 0

    def get(self, path):
        for attempt in range(4):
            time.sleep(0.7 if attempt == 0 else 5 * attempt)
            self.calls += 1
            try:
                req = urllib.request.Request(BASE + path, headers={"X-API-Key": self.key, "User-Agent": "zulu-netmetrics"})
                with urllib.request.urlopen(req, timeout=30) as r:
                    body = r.read().decode("utf-8")
                if body[:1] in "[{":
                    return json.loads(body)
            except (OSError, ValueError):
                pass
        raise RuntimeError("radarvan %s failed" % path)

    def cached(self, name, path, keep):
        f = os.path.join(self.cache, name)
        if keep and os.path.exists(f):
            with open(f) as fh:
                return json.load(fh)
        d = self.get(path)
        tmp = f + ".tmp"
        with open(tmp, "w") as fh:
            json.dump(d, fh)
        os.replace(tmp, f)
        return d


def load(rv, today):
    """Games in the window (with details where radarvan has them), bracket, tournament results."""
    os.makedirs(os.path.join(rv.cache, "det"), exist_ok=True)
    dates = rv.get("/api/dates/")
    first = (today - dt.timedelta(days=WINDOW_DAYS)).isoformat()
    games = []
    for day in sorted(d for d in dates if d >= first):
        keep = day < (today - dt.timedelta(days=REFETCH_DAYS)).isoformat()
        for g in rv.cached("day_%s.json" % day, "/api/matches/by_date/%s?exclude_dev=true" % day, keep).get("matches", []):
            comp = g.get("composition") or {}
            ps = [p for p in g.get("players", []) if p.get("general", -1) >= 0]
            if comp.get("numComputers", 0) > 0 or comp.get("numHumans", 0) < 2 or g.get("incomplete") or not any(p["won"] for p in ps):
                continue
            try:
                d = rv.cached("det/%s.json" % g["id"], "/api/details/%s" % g["id"], True)
                # Only what compute reads: a full details file is mostly curves it never looks at.
                g["details"] = {k: d.get(k) for k in ("player_summary", "killEvents", "mapEvents", "first_blood")}
            except RuntimeError:
                g["details"] = None  # try again tomorrow
            games.append(g)
    return games, rv.get("/api/bracket"), rv.get("/api/tournaments"), rv.get("/api/tournament_results/")


def champions(bracket, tournaments, results, state_file):
    """{name: [(key, why)]} for the tournament champions. The 1v1 champion is remembered,
    so a new bracket that has not finished yet does not take the honor away."""
    out = C.defaultdict(list)
    state = {}
    if os.path.exists(state_file):
        state = json.load(open(state_file))
    if bracket and bracket.get("champion"):
        state["champ1v1"] = {"name": canon(bracket["champion"]), "why": bracket.get("runner_up") and "beat %s in the final" % canon(bracket["runner_up"]) or ""}
        json.dump(state, open(state_file, "w"))
    if state.get("champ1v1"):
        out[state["champ1v1"]["name"]].append(("champ1v1", state["champ1v1"]["why"]))
    # radarvan lists tournaments newest first; results name them by slug.
    done2v2 = [t["slug"] for t in tournaments if (t.get("format") or "").startswith("2v2") and t.get("status") == "complete"]
    for res in results:
        t = res.get("tournament") or {}
        if not done2v2 or t.get("name") != done2v2[0]:
            continue
        teams = {frozenset(canon(n) for n in team): team for team in t.get("teams", [])}
        w, l = C.Counter(), C.Counter()
        for mu in res.get("matchups", []):
            for g in mu.get("matches", []):
                sides = C.defaultdict(list)
                for p in g["players"]:
                    if p.get("team", -1) >= 1:
                        sides[p["team"]].append(p)
                won = {tn: any(p["won"] for p in ps) for tn, ps in sides.items()}
                if sorted(won.values()) != [False, True]:
                    continue
                for tn, ps in sides.items():
                    key = frozenset(canon(p["name"]) for p in ps)
                    if key in teams:
                        (w if won[tn] else l)[key] += 1
        if w:
            best = max(w, key=lambda k: (w[k], -l[k]))
            for n in best:
                out[n].append(("champ2v2", "with %s, %d-%d" % (" & ".join(sorted(best - {n})), w[best], l[best])))
    return out


def compute(games, champs):
    """{name: [{"k": key, "why": text}]}: one holder per honor among the regulars."""
    per = C.defaultdict(list)          # player -> [(game, general, won)]
    for g in sorted(games, key=lambda g: g["timestamp"]):
        comp = g.get("composition") or {}
        g["cat"] = "FFA" if comp.get("isFfa") else comp.get("category") or ("1v1" if comp.get("is1V1") else "team")
        for p in g["players"]:
            if p.get("general", -1) >= 0:
                per[canon(p["name"])].append((g, p["general"], bool(p["won"])))
    reg = {n for n, l in per.items() if len(l) >= MIN_GAMES}
    med = {}
    for cat in {g["cat"] for g in games}:
        med[cat] = st.median(g["duration_minutes"] for g in games if g["cat"] == cat)
    played = {n: len(per[n]) for n in reg}
    detail_games = C.Counter()
    stat = C.defaultdict(C.Counter)
    for g in games:
        d = g.get("details")
        if not d:
            continue
        team = {}
        for p in d.get("player_summary") or []:
            n = canon(p["Name"])
            team[n] = p.get("Team")
            detail_games[n] += 1
            for t, v in (p.get("UnitsCreated") or {}).items():
                u = unit(t)
                if any(h in u for h in HEROES):
                    stat["heroes"][n] += v["Count"]
                if u in RARE:
                    stat["rare"][n] += v["Count"]
                if u == "GLAVehicleScudLauncher":
                    stat["scud"][n] += v["Count"]
                if u == "ChinaVehicleNukeLauncher":
                    stat["nukecannon"][n] += v["Count"]
            for pw, c in (p.get("PowersUsed") or {}).items():
                if any(s in pw for s in SUPERWEAPONS):
                    stat["bigred"][n] += c
        for e in d.get("killEvents") or []:
            if not e.get("killerPlayer") or e.get("killerPlayer") == e.get("victimPlayer"):
                continue
            k, killer, value = canon(e["killerPlayer"]), unit(e.get("killer") or ""), e.get("value") or 0
            if "Terrorist" in killer:
                stat["terrorist"][k] += value
                stat["terrorist_n"][k] += 1
            if any(h in killer for h in HEROES):
                stat["hero_value"][k] += value
            if killer == "AmericaTankPaladin":
                stat["paladin"][k] += value
            if "Anthrax" in killer:
                stat["plague"][k] += 1
            if killer in DEFENSES:
                stat["holdline"][k] += 1
        builds = {}
        for e in d.get("mapEvents") or []:
            n, spot = canon(e.get("playerName") or ""), (round(e["x"]), round(e["y"]))
            if e["kind"] == "build":
                builds[spot] = n
            elif e["kind"] == "capture":
                if e.get("name", "").startswith("Tech"):
                    stat["oil"][n] += 1
                orig = builds.get(spot)
                if orig and orig != n and team.get(orig) != team.get(n):
                    stat["takeover"][n] += 1
        fb = (d.get("first_blood") or {}).get("attacker")
        if fb:
            stat["firststrike"][canon(fb)] += 1

    def pick(scores):
        """The single best (score, games) among regulars with a positive score."""
        cand = [(v, played[n], n) for n, v in scores.items() if n in reg and v > 0]
        return max(cand)[2] if cand else None

    out = C.defaultdict(list)
    for n, lst in champs.items():
        out[n] += [{"k": k, "why": why} for k, why in lst]

    def award(key, scores, why):
        n = pick(scores)
        if n:
            out[n].append({"k": key, "why": why(n)})

    streak = {n: next((i for i, (_, _, w) in enumerate(reversed(per[n])) if not w), len(per[n])) for n in reg}
    award("streak", {n: v for n, v in streak.items() if v >= 3}, lambda n: "%d wins in a row" % streak[n])
    gens = {}
    for n in reg:
        by = C.defaultdict(lambda: [0, 0])
        for _, gen, w in per[n]:
            by[gen][0] += w
            by[gen][1] += 1
        best = max(((w / t, t, gen) for gen, (w, t) in by.items() if t >= MIN_GAMES), default=None)
        if best:
            gens[n] = best
    award("general", {n: v[0] for n, v in gens.items()},
          lambda n: "%s, %d%% of %d games" % (GEN.get(gens[n][2], "?"), round(gens[n][0] * 100), gens[n][1]))
    maps = {}
    for n in reg:
        by = C.defaultdict(lambda: [0, 0])
        for g, _, w in per[n]:
            m = g["map"].split("/")[-1]
            by[m][0] += w
            by[m][1] += 1
        best = max(((w / t, t, m) for m, (w, t) in by.items() if t >= 6), default=None)
        if best:
            maps[n] = best
    award("map", {n: v[0] for n, v in maps.items()},
          lambda n: "%s, won %d of %d" % (maps[n][2].replace("[rank] ", "").title(), round(maps[n][0] * maps[n][1]), maps[n][1]))
    duel = {n: [(w) for g, _, w in per[n] if g["cat"] == "1v1"] for n in reg}
    award("duelist", {n: sum(v) / len(v) for n, v in duel.items() if len(v) >= MIN_GAMES},
          lambda n: "won %d of %d 1v1s" % (sum(duel[n]), len(duel[n])))
    long_wins = {n: sum(1 for g, _, w in per[n] if w and g["duration_minutes"] > 1.5 * med[g["cat"]]) for n in reg}
    award("ironwill", long_wins, lambda n: "%d long game wins" % long_wins[n])
    quick = {n: sum(1 for g, _, w in per[n] if w and g["duration_minutes"] < 0.75 * med[g["cat"]]) for n in reg}
    # A rate over a handful of games is noise: Fastest Closer needs twice the regular's games.
    award("closer", {n: quick[n] / played[n] for n in reg if played[n] >= 2 * MIN_GAMES},
          lambda n: "%d quick wins in %d games" % (quick[n], played[n]))

    dg = {n: detail_games[n] for n in reg if detail_games[n] >= MIN_GAMES}
    rate = lambda key: {n: stat[key][n] / dg[n] for n in dg}
    award("terrorist", {n: stat["terrorist"][n] for n in dg},
          lambda n: "%d kills, $%s destroyed" % (stat["terrorist_n"][n], format(stat["terrorist"][n], ",")))
    award("hero", {n: stat["hero_value"][n] / stat["heroes"][n] for n in dg if stat["heroes"][n] >= MIN_GAMES},
          lambda n: "$%d destroyed per hero, %d heroes" % (stat["hero_value"][n] / stat["heroes"][n], stat["heroes"][n]))
    award("collector", {n: stat["rare"][n] for n in dg}, lambda n: "%d rare units built" % stat["rare"][n])
    award("takeover", rate("takeover"), lambda n: "%d enemy buildings captured" % stat["takeover"][n])
    award("oil", rate("oil"), lambda n: "%d tech buildings captured" % stat["oil"][n])
    award("firststrike", rate("firststrike"), lambda n: "first blood in %d of %d games" % (stat["firststrike"][n], dg[n]))
    award("bigred", {n: stat["bigred"][n] for n in dg}, lambda n: "%d superweapon launches" % stat["bigred"][n])
    award("paladin", rate("paladin"), lambda n: "$%s destroyed with Paladins" % format(stat["paladin"][n], ","))
    award("scud", rate("scud"), lambda n: "%d Scud Launchers built" % stat["scud"][n])
    award("nukecannon", rate("nukecannon"), lambda n: "%d Nuke Cannons built" % stat["nukecannon"][n])
    award("plague", rate("plague"), lambda n: "%d anthrax kills" % stat["plague"][n])
    award("holdline", rate("holdline"), lambda n: "%d kills by base defenses" % stat["holdline"][n])
    return dict(out)


def honors_sql(honors):
    rows = ",\n".join("(%s,%s,UTC_TIMESTAMP())" % (q(n), q(json.dumps(h, separators=(",", ":")))) for n, h in sorted(honors.items()))
    return ("SET NAMES utf8mb4;\nUSE zulumetrics;\nSTART TRANSACTION;\nDELETE FROM player_honors;\n"
            + ("INSERT INTO player_honors VALUES\n" + rows + ";\n" if rows else "") + "COMMIT;\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cache", default="/var/lib/netmetrics/radarvan")
    ap.add_argument("--dry-run", action="store_true", help="print the honors instead of writing them")
    a = ap.parse_args()
    key = os.environ.get("RADARVAN_KEY")
    if not key and os.path.exists("/opt/netmetrics/radarvan.env"):
        key = dict(l.strip().split("=", 1) for l in open("/opt/netmetrics/radarvan.env") if "=" in l).get("RADARVAN_KEY")
    if not key:
        sys.exit("honors: no RADARVAN_KEY")
    rv = Radarvan(key, a.cache)
    os.makedirs(a.cache, exist_ok=True)
    games, bracket, tournaments, results = load(rv, dt.datetime.now(dt.timezone.utc).date())
    honors = compute(games, champions(bracket, tournaments, results, os.path.join(a.cache, "champions.json")))
    if a.dry_run:
        print(json.dumps(honors, indent=1))
    else:
        subprocess.run(DB_CMD, input=honors_sql(honors), text=True, check=True)
    print("honors: %d games, %d radarvan calls, %d holders" % (len(games), rv.calls, len(honors)), file=sys.stderr)


if __name__ == "__main__":
    main()
