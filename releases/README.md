The `release.sh` and the `test.sh` scripts will populate this directory.

`scripts/windows-build.sh` additionally writes two stable-named Windows binaries
here for the manual test VM (see `WINDOWS-VM.md`):

- `moor.exe` — built from the current branch
- `moor-master.exe` — built from `master`

The VM shares this directory and puts it on the guest's `PATH`, so inside Windows
`moor` and `moor-master` run the latest build of each. Rerunning
`scripts/windows-build.sh` on the host refreshes them in place.
