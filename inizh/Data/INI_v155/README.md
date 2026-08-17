# zulu 1.5.5+ INI data

Effective gameplay data for zulu releases 1.5.5 and later, whose `Zulu.big`
carries the TheSuperHackers community balance patch (imported into
GeneralsGameCode `assets/Data/INI` by commit `07a5725a2`).

Contents mirror what the engine actually loads for the stores cncstats uses:

- `Object/FactionUnit.ini` — verbatim retail 1.04 file, extracted from
  `INIZH.big`. It is the one retail Object file the patch does not blank,
  so it still loads first and its objects take the first template IDs.
- `Object/<subdirs>/` — the patch's nested per-unit tree, copied from
  GeneralsGameCode `assets/Data/INI/Object/`. The 43 blank top-level
  `Object/*.ini` stubs that shadow the other retail files are omitted;
  they contribute nothing.
- `Upgrade.ini`, `SpecialPower.ini` — from `assets/Data/INI/`.

Load order is replicated by `iniparse.loadObjects`: top-level files first,
then subdirectory files, both sorted case-insensitively by full path with
`\` separators (the engine's `INI::loadDirectory` over a `FilenameList`).
Redefinitions overwrite in place without taking a new ID.

Verified byte-identical against the `Zulu.big` inside `Zulu_Setup.exe`
built 2026-08-15 (v1.5.6).

To refresh after a future data change: re-copy `Upgrade.ini`,
`SpecialPower.ini`, and the `Object` subdirectories from
`GeneralsGameCode/assets/Data/INI/`; `FactionUnit.ini` only changes if the
retail base or the patch's blanking set changes.
