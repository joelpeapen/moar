# Data Races

`moor.sh` and `test.sh` both enable the race detector and a detected race is
written to a `moor-race-report.*` file in the repo root.

Those files are untracked and in your way on purpose. Don't add them to
`.gitignore`.

## Triage

If any such file is found, diagnose it before finishing whatever else you are
doing. Work out which of these it is, since that decides what to do about it:

- A race the current branch introduced.
- A race that master has as well.
- Stale, describing code that no longer exists.

Answer that by reading the code the backtraces point at, and asking whether
those accesses can still happen unsynchronized today. Line numbers alone won't
tell you: they drift while a bug survives, and a lock added in some caller
fixes a race without touching any line in the report.

To tell your own race apart from one master already has, check whether the
commit that introduced the racing code is an ancestor of master.

Several report files, and several stanzas within one file, are often the same
race reached through different call paths. Diagnose before concluding how many
bugs you have.

If the report is stale, explain how you concluded that, and offer to delete it.

## Fixing

If the race condition was introduced by the current branch, or blocks it, fix
it immediately. On the branch you are already on, no need for a separate one.

If it already existed on master and doesn't block the current branch, file an
issue ticket.

Race fixes are exempt from the "reproduce with a failing test" rule in
AGENTS.md. Committed race tests are unreliable, and a repro that loses the race
often enough to be trustworthy tends to be too slow for the 20s test timeout.
Write a throwaway repro instead, one that reports the race on nearly every run,
confirm it goes clean with the fix, then delete it without committing. Say in
the commit message that you verified it that way. Don't pause to have the repro
reviewed, it isn't a test we are keeping.

A clean `-race` run proves nothing on its own, the detector only reports races
it happens to observe. So keep the report file until the fix is verified, and
delete it after.
