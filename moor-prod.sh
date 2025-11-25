#!/bin/bash

# Build pager and run it, this script should behave just
# like the production binary. Faster than moor.sh by not
# having race detection enabled.

set -e -o pipefail

MYDIR="$(
    cd "$(dirname "$0")"
    pwd
)"
cd "$MYDIR"

rm -f moor

./build.sh 1>&2

./moor "$@"
