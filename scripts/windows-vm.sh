#!/bin/bash

# Manage the VirtualBox VM used for manual Windows testing of moor. See
# WINDOWS-VM.md for the surrounding workflow (installing Windows, testing).
#
# Behaviour depends on what already exists:
#
#   * No VM yet, ISO given      -> create the VM and boot into the installer.
#   * No VM yet, no ISO          -> explain where to download an ISO.
#   * VM exists, no ISO          -> start it (or note it's already running).
#   * VM exists, ISO given       -> refuse: it's ambiguous, so say what to do.
#
# The Windows install itself is a manual wizard; this script just gets you to
# it and back.
#
# Usage:
#   scripts/windows-vm.sh path/to/Win11_Eval.iso   # first time: create + boot
#   scripts/windows-vm.sh                          # later: start the VM

set -e -o pipefail

VM=moor-windows-test
SHARE=windows-vm-io
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IO_DIR="${REPO_ROOT}/.windows-vm-io"
ISO_URL="https://www.microsoft.com/en-us/evalcenter/download-windows-11-enterprise"

ISO="$1"

# Keep the guest's copy of the bootstrap script current. Cheap, and it means the
# guest never runs a stale one.
refresh_bootstrap() {
    mkdir -p "${IO_DIR}"
    cp "${REPO_ROOT}/scripts/windows-vm-bootstrap.bat" "${IO_DIR}/bootstrap.bat"
}

if ! command -v VBoxManage >/dev/null; then
    echo "VBoxManage not found. Install VirtualBox first." >&2
    exit 1
fi

refresh_bootstrap

vm_exists() { VBoxManage list vms | grep -q "\"${VM}\""; }
vm_running() { VBoxManage list runningvms | grep -q "\"${VM}\""; }

# Echo VirtualBox's name for the VM's current state: "running", "poweroff",
# "saved" or "aborted-saved" are the ones that come up here.
vm_state() {
    VBoxManage showvminfo "${VM}" --machinereadable |
        sed -n 's/^VMState="\(.*\)"$/\1/p'
}

# Point an existing VM at the one writable share this script now creates. VMs made
# by earlier versions have a read-only "releases" share instead, which the guest
# can't write output back through, and swapping it beats reinstalling Windows.
# Only valid while the VM is powered off; a running one would need --transient.
ensure_share() {
    local info
    info="$(VBoxManage showvminfo "${VM}" --machinereadable)"

    if grep -q "^SharedFolderPathMachineMapping[0-9]*=\"${IO_DIR}\"$" <<<"${info}"; then
        return
    fi

    echo "Pointing the shared folder at ${IO_DIR}..."
    local name
    for name in releases "${SHARE}"; do
        if grep -q "^SharedFolderNameMachineMapping[0-9]*=\"${name}\"$" <<<"${info}"; then
            VBoxManage sharedfolder remove "${VM}" --name "${name}"
        fi
    done

    VBoxManage sharedfolder add "${VM}" \
        --name "${SHARE}" \
        --hostpath "${IO_DIR}" \
        --automount

    printf 'Run \\\\VBOXSVR\\%s\\bootstrap.bat in the guest to put it on the PATH.\n' \
        "${SHARE}"
}

# The VM is already there: start it, or refuse to reconcile it with an ISO.
if vm_exists; then
    if [ -n "${ISO}" ]; then
        echo "VM \"${VM}\" already exists, so the ISO argument is ambiguous." >&2
        echo "" >&2
        echo "Either recreate it from scratch..." >&2
        echo "  VBoxManage unregistervm \"${VM}\" --delete" >&2
        echo "" >&2
        echo "... or run with no ISO argument to just start it." >&2
        exit 1
    fi

    if vm_running; then
        echo "VM \"${VM}\" is already running."
        exit 0
    fi

    # Resuming saved state brings back the guest's evaluation shutdown timer
    # along with everything else, and that timer is usually nearly elapsed — so
    # a resumed guest tends to power off again within minutes. A cold boot
    # restarts the timer and buys a full hour. See WINDOWS-VM.md.
    case "$(vm_state)" in
        saved | aborted-saved)
            echo "VM \"${VM}\" has saved state, which resumes with its evaluation"
            echo "shutdown timer nearly elapsed — expect to be powered off within"
            echo "minutes. Discarding the state instead cold boots it, which is"
            echo "worth a full hour of testing."
            echo

            # Non-interactive callers get the cold boot: it's the useful default,
            # and read would otherwise fail the script under set -e.
            reply=""
            if [ -t 0 ]; then
                read -r -p "Discard saved state and cold boot? [Y/n] " reply
            fi

            case "${reply}" in
                [Nn]*) echo "Resuming saved state." ;;
                *) VBoxManage discardstate "${VM}" ;;
            esac
            ;;
    esac

    if [ "$(vm_state)" = poweroff ]; then
        ensure_share
    fi

    echo "Starting \"${VM}\"..."
    VBoxManage startvm "${VM}" --type gui
    echo
    echo "An elevated cmd prompt opens by itself about a minute after the desktop"
    echo "appears — an empty desktop before then is not a failure. Waiting for it"
    echo "beats opening one through Task Manager."
    exit 0
fi

# No VM and no ISO: we can't build one, so point at the download.
if [ -z "${ISO}" ]; then
    echo "No \"${VM}\" VM yet, and no ISO given to create one."
    echo
    echo "Download a Windows 11 Enterprise 90-day evaluation ISO (free, no key):"
    echo "  ${ISO_URL}"
    echo
    echo "Prefer the LTSC flavor if available."
    echo
    echo "Then run: $0 path/to/Win11_Eval.iso"
    exit 0
fi

if [ ! -f "${ISO}" ]; then
    echo "ISO not found: ${ISO}" >&2
    exit 1
fi

# Create the VM. Windows 11 refuses to install without EFI firmware and a TPM
# 2.0 device, so those flags are not optional. We keep the default NAT adapter:
# moor binaries arrive over the shared folder rather than the network, but a
# working connection lets Windows setup get past its "connect to a network"
# screen instead of looping, and lets you pull down extras (e.g. Windows
# Terminal) inside the guest later. Bidirectional clipboard is enabled in the
# hope of pasting into the guest console, but don't count on it: in audit mode
# Ctrl+V there does nothing. Use scripts/windows-vm-run.sh instead.
VBoxManage createvm --name "${VM}" --ostype Windows11_64 --register
VBoxManage modifyvm "${VM}" \
    --memory 8192 --cpus 6 \
    --firmware efi --tpm-type 2.0 \
    --graphicscontroller vboxsvga --vram 128 \
    --nic1 nat \
    --clipboard-mode bidirectional

# Put the disk next to the VM config, wherever VirtualBox placed that.
VM_DIR="$(dirname "$(VBoxManage showvminfo "${VM}" --machinereadable | sed -n 's/^CfgFile="\(.*\)"$/\1/p')")"
DISK="${VM_DIR}/${VM}.vdi"

VBoxManage createmedium disk --filename "${DISK}" --size 65536 --format VDI
VBoxManage storagectl "${VM}" --name SATA --add sata --controller IntelAhci
VBoxManage storageattach "${VM}" --storagectl SATA --port 0 --device 0 \
    --type hdd --medium "${DISK}"
VBoxManage storageattach "${VM}" --storagectl SATA --port 1 --device 0 \
    --type dvddrive --medium "${ISO}"

# Permanent shared folder, and the guest's only channel to the host: it carries
# the built binaries in, batch files in, and command output back out. Writable, so
# the guest can answer — see scripts/windows-vm-run.sh. Allowed here because the
# VM is powered off; a running VM would need --transient instead.
VBoxManage sharedfolder add "${VM}" \
    --name "${SHARE}" \
    --hostpath "${IO_DIR}" \
    --automount

echo "Created \"${VM}\"."
echo
echo "About to boot into the Windows installer. On first boot the VM briefly"
echo "shows \"Press any key to boot from CD or DVD...\" — click into the VM"
echo "window and press a key within those few seconds, or the boot fails and"
echo "you'll have to reset the VM and retry."
echo
echo "See WINDOWS-VM.md for the rest of the install — OOBE has a gotcha worth"
echo "reading before you get there."
echo
if [ -t 0 ]; then
    read -r -p "Press RETURN when you're ready to watch the VM window... "
fi

VBoxManage startvm "${VM}" --type gui
