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
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RELEASES_DIR="${REPO_ROOT}/releases"
ISO_URL="https://www.microsoft.com/en-us/evalcenter/download-windows-11-enterprise"

ISO="$1"

if ! command -v VBoxManage >/dev/null; then
    echo "VBoxManage not found. Install VirtualBox first." >&2
    exit 1
fi

vm_exists() { VBoxManage list vms | grep -q "\"${VM}\""; }
vm_running() { VBoxManage list runningvms | grep -q "\"${VM}\""; }

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

    echo "Starting \"${VM}\"..."
    VBoxManage startvm "${VM}" --type gui
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
# Terminal) inside the guest later. Bidirectional clipboard lets you paste test
# commands from the host into the guest console (it also needs VBoxTray running
# inside the guest, which Guest Additions handles).
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

# Permanent shared folder for the built binaries. Read-only because the guest
# only runs them, never writes back. Allowed here because the VM is powered off;
# a running VM would need --transient instead.
VBoxManage sharedfolder add "${VM}" \
    --name releases \
    --hostpath "${RELEASES_DIR}" \
    --readonly \
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
read -r -p "Press RETURN when you're ready to watch the VM window... "
VBoxManage startvm "${VM}" --type gui
