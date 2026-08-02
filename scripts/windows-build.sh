#!/bin/bash

# Cross-compile Windows moor binaries under stable names for the test VM:
#
#   .windows-vm-io/moor.exe          <- the current branch
#   .windows-vm-io/moor-master.exe   <- master
#
# That directory is shared into the guest and is on the guest's PATH, so after
# running this you just type `moor` or `moor-master` in the VM to run the latest
# of each. Re-run this whenever you want to refresh those binaries.
#
# See WINDOWS-VM.md for the full VM workflow, and .windows-vm-io/README.md for
# what else lives in there.

set -e -o pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IO_DIR="${REPO_ROOT}/.windows-vm-io"

# Cross-compile moor for Windows from the git tree at $1 and echo the path of
# the version-stamped binary that build.sh produced there.
build_windows() {
    local tree="$1"
    mkdir -p "${tree}/releases"
    ( cd "${tree}" && GOOS=windows GOARCH=amd64 ./build.sh >/dev/null )

    local version
    version="$(git -C "${tree}" describe --tags --dirty --always)"
    echo "${tree}/releases/moor-${version}-windows-amd64.exe"
}

# Current branch -> moor.exe.
mkdir -p "${IO_DIR}"
BRANCH="$(git -C "${REPO_ROOT}" rev-parse --abbrev-ref HEAD)"
echo "Building ${BRANCH} -> moor.exe ..."
cp "$(build_windows "${REPO_ROOT}")" "${IO_DIR}/moor.exe"

# master -> moor-master.exe, built in a throwaway worktree so the current
# checkout (and any uncommitted work in it) is left untouched.
WORKTREE="$(mktemp -d)/moor-master"
cleanup() {
    git -C "${REPO_ROOT}" worktree remove --force "${WORKTREE}" 2>/dev/null || true
}
trap cleanup EXIT

echo "Building master -> moor-master.exe ..."
# --detach so this still works when master is the current branch: git refuses to
# check out the same branch in two worktrees, but a detached HEAD at its commit
# is fine.
git -C "${REPO_ROOT}" worktree add --detach "${WORKTREE}" master >/dev/null
cp "$(build_windows "${WORKTREE}")" "${IO_DIR}/moor-master.exe"

echo
echo "Done. In the VM (.windows-vm-io/ is on the guest PATH):"
echo "  moor          # ${BRANCH}"
echo "  moor-master   # master"
