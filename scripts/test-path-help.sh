#!/bin/bash

# Verify the PAGER= suggestion is correct in "moor --help" output.
#
# Ref: https://github.com/walles/moor/issues/88

set -e -o pipefail

MOOR="$(realpath "$1")"
if ! [ -x "$MOOR" ]; then
    echo ERROR: Not executable: "$MOOR"
    exit 1
fi

echo Testing PAGER= suggestion in moor --help output...

WORKDIR="$(mktemp -d -t moor-path-help-test.XXXXXXXX)"

# Put a symlink to $MOOR first in the $PATH
ln -s "$MOOR" "$WORKDIR/moor"
echo "moor" >"$WORKDIR/expected"

# Extract suggested PAGER value from moor --help
unset PAGER
PATH="$WORKDIR" PAGER="" MOOR="" moor --help > "$WORKDIR/help-printout.txt"
cat "$WORKDIR/help-printout.txt" | grep '^  PAGER' | sed 's/.* //' | sed 's/\x1b.*//' >"$WORKDIR/actual"

# Ensure it matches the symlink we have in $PATH
cd "$WORKDIR"

if diff -u actual expected ; then
    exit 0
fi

# Diff output already printed as part of the if statement ^
echo
env | sort
echo
echo Failing help text:
cat "$WORKDIR/help-printout.txt"
echo
echo "Actual:   <$(cat "$WORKDIR/actual")>"
echo "Expected: <$(cat "$WORKDIR/expected")>"
echo "Workdir:  <$(pwd)>"
exit 1
