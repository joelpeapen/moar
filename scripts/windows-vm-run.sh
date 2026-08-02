#!/bin/bash

# Run a non-interactive command inside the Windows test VM and print its output
# here on the host. See WINDOWS-VM.md for the VM itself.
#
# The guest can't be driven directly: VBoxManage guestcontrol refuses to log into
# the audit-mode Administrator account, and keystrokes only go one way, so nothing
# comes back. Both gaps are bridged by the shared .windows-vm-io directory:
#
#   1. Write the command into runcmd.bat, which the guest can see.
#   2. Type "runcmd" at the guest's prompt — letters only, so no keyboard layout
#      can mangle it.
#   3. Wait for the guest to write stdout, stderr and the exit code back.
#
# Needs the VM running with an admin cmd prompt focused. Interactive programs are
# not usable this way — moor included — because nothing reads their output until
# they exit, and there's no terminal to drive.
#
# Three batch quirks leak through into the command you pass: a bare `%` is eaten by
# the parser, so `echo 100%` prints `100`; `for` wants `%%i` where a prompt would
# take `%i`; and a command that is itself a .bat or .cmd file needs an explicit
# `call`, because otherwise it replaces this script's batch context and the run
# ends in "cannot find the batch label specified - body" with no output.
#
# Usage:
#   scripts/windows-vm-run.sh 'ver'
#   scripts/windows-vm-run.sh 'cscript //nologo C:\Windows\System32\slmgr.vbs /dlv'

set -e -o pipefail

VM=moor-windows-test
SHARE=windows-vm-io
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IO_DIR="${REPO_ROOT}/.windows-vm-io"
BAT_FILE="${IO_DIR}/runcmd.bat"
ECHO_FILE="${IO_DIR}/runcmd.txt"
OUT_FILE="${IO_DIR}/out.txt"
TIMEOUT_SECONDS=60

# Unique per run, so output that an earlier timed-out run appends later can never
# be mistaken for this run's answer.
SENTINEL="__DONE_$$_${RANDOM}__"

CMD="$1"

if [ -z "${CMD}" ]; then
    echo "Usage: $0 'command to run in the guest'" >&2
    exit 1
fi

if ! command -v VBoxManage >/dev/null; then
    echo "VBoxManage not found. Install VirtualBox first." >&2
    exit 1
fi

if ! VBoxManage list runningvms | grep -q "\"${VM}\""; then
    echo "VM \"${VM}\" isn't running; start it with scripts/windows-vm.sh" >&2
    exit 1
fi

# The guest reaches this directory through a share set up when the VM starts, and
# without it nothing below can work. Checking beats timing out with a guess.
if ! VBoxManage showvminfo "${VM}" --machinereadable |
        grep -q "^SharedFolderPathMachineMapping[0-9]*=\"${IO_DIR}\"$"; then
    echo "VM \"${VM}\" doesn't share ${IO_DIR}." >&2
    echo "Start it with scripts/windows-vm.sh, which sets that up." >&2
    exit 1
fi

# A batch file still sitting here means an earlier run never finished. Its command
# may yet be running in the guest, and two commands appending to one output file
# would interleave, so stop rather than guess.
if [ -e "${BAT_FILE}" ]; then
    echo "${BAT_FILE} is left over from an unfinished run." >&2
    echo "If the guest isn't still running it, delete it and try again." >&2
    exit 1
fi

mkdir -p "${IO_DIR}"
rm -f "${OUT_FILE}"

# The guest console only ever sees "runcmd", so print the command there too — via
# `type` of a plain file, because passing it to `echo` would let any > | or & in it
# redirect for real.
printf '%s\r\n' "${CMD}" > "${ECHO_FILE}"

# The command goes on its own line after a label, so cmd.exe sees it exactly as
# you'd type it at a prompt; wrapping it in ( ) instead would let any ) inside it
# close the block early and detach the redirect. The exit code is saved before
# anything else runs, because echo resets it. CRLF endings throughout, cmd.exe
# being unforgiving about Unix ones.
{
    printf '@echo off\r\n'
    printf 'echo.\r\n'
    printf 'type \\\\VBOXSVR\\%s\\runcmd.txt\r\n' "${SHARE}"
    printf 'call :body > \\\\VBOXSVR\\%s\\out.txt 2>&1\r\n' "${SHARE}"
    printf 'set RC=%%errorlevel%%\r\n'
    printf 'echo EXIT=%%RC%%\r\n'
    printf 'echo EXIT=%%RC%% >> \\\\VBOXSVR\\%s\\out.txt\r\n' "${SHARE}"
    printf 'echo %s >> \\\\VBOXSVR\\%s\\out.txt\r\n' "${SENTINEL}" "${SHARE}"
    printf 'goto :eof\r\n'
    printf ':body\r\n'
    printf '%s\r\n' "${CMD}"
} > "${BAT_FILE}"

VBoxManage controlvm "${VM}" keyboardputstring runcmd
VBoxManage controlvm "${VM}" keyboardputscancode 1c 9c  # Return, make and break

for _ in $(seq "${TIMEOUT_SECONDS}"); do
    if [ -f "${OUT_FILE}" ] && grep -q "${SENTINEL}" "${OUT_FILE}"; then
        grep -v "${SENTINEL}" "${OUT_FILE}" || true
        rm -f "${OUT_FILE}" "${BAT_FILE}" "${ECHO_FILE}"
        exit 0
    fi

    sleep 1
done

echo "Gave up after ${TIMEOUT_SECONDS}s waiting for the guest to answer." >&2
echo "Is a cmd prompt focused in the VM window? One opens by itself about a" >&2
echo "minute after the desktop appears, so a freshly booted guest isn't ready yet." >&2

if [ -s "${OUT_FILE}" ]; then
    echo "Output so far:" >&2
    cat "${OUT_FILE}" >&2
fi

exit 1
