# Testing moor on Windows in a VM

Some moor behaviour can only be verified interactively on real Windows: how the
keyboard and terminal modes behave in `cmd.exe`, mouse handling, and anything
that differs from Unix. This is a walkthrough for setting up a throwaway Windows
VM in [VirtualBox](https://www.virtualbox.org/) on macOS or Linux to do that
testing.

The deterministic parts — creating and starting the VM — live in
`scripts/windows-vm.sh`. The rest is prose because it can't be scripted: the
Windows install is an interactive wizard that would need a lot of
`Autounattend.xml` machinery to automate, and the actual testing is you pressing
keys and watching.

## Getting a Windows image

Download a **Windows 11 Enterprise 90-day evaluation ISO** from the [Microsoft
Evaluation
Center](https://www.microsoft.com/en-us/evalcenter/download-windows-11-enterprise).
It is free, needs no product key, and is meant exactly for this kind of
temporary testing.

Prefer the **LTSC** variant when the page offers it: it is leaner, doesn't nag
about feature updates, and ships without the Microsoft Store — so `cmd.exe`
opens in the legacy console host rather than Windows Terminal, which is the
classic environment for terminal-mode quirks. Install Windows Terminal
separately if you also want to test that path.

The Windows 11 **N** editions (shipped without Windows Media Player, for EU
regulatory reasons) are fine: the console has no dependency on media components,
so moor behaves identically.

## Creating the VM

```bash
scripts/windows-vm.sh ~/Downloads/Win11_Eval.iso
```

This creates a VM named `moor-windows-test` with a 64 GB disk, attaches the
evaluation ISO, wires a shared folder (see [Getting moor.exe into the
VM](#getting-moorexe-into-the-vm) below), and boots the VM's GUI into the
Windows installer. Windows 11 refuses to install without EFI firmware and a TPM
2.0 device, so the script sets those up for you.

Run the same script with no argument later to start an existing VM, and with no
argument and no VM to get a download link.

## Installing Windows

The script pauses before booting and reminds you of a timing-critical step: on
first boot the VM briefly shows **"Press any key to boot from CD or DVD…"**, and
you must click into the VM window and press a key within those few seconds, or
the boot fails and you have to reset and retry. After that, click through the
installer. (We run with a GUI, never headless, because the whole point is typing
at it.)

If Setup complains "This PC can't run Windows 11" despite the TPM above, open a
command prompt in the installer with <kbd>Shift</kbd>+<kbd>F10</kbd> and add the
well-known bypass keys, then go back one screen and retry:

```cmd
reg add HKLM\SYSTEM\Setup\LabConfig /v BypassTPMCheck /t REG_DWORD /d 1
reg add HKLM\SYSTEM\Setup\LabConfig /v BypassSecureBootCheck /t REG_DWORD /d 1
```

### Getting past OOBE with audit mode

After Windows copies files and reboots into the out-of-box experience (OOBE — the
"let's set up your account" phase), Windows 11 24H2 tends to crash here and loop
on a blue **"Why did my PC restart?"** screen that insists you connect to a
network for an update. The usual offline-account escapes don't rescue this
particular screen: `oobe\bypassnro` is disabled in 24H2, and
`start ms-cxh:localonly` fails with "Class not registered". Adding or removing a
network adapter doesn't help either — it's an OOBE crash, not really a network
problem.

Skip OOBE entirely instead. On any OOBE screen press
<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F3</kbd> (on a Mac keyboard,
<kbd>Fn</kbd>+<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>F3</kbd>) to enter **audit
mode**: Windows reboots straight to the desktop as the built-in Administrator. A
*System Preparation Tool* (Sysprep) dialog opens on top — just close it, don't
run sysprep. Audit mode survives reboots, and a local Administrator desktop is
all you need to run moor in `cmd.exe`, so this is a perfectly good place to stop.
You never have to finish OOBE or create a user account.

Audit mode is bare: the taskbar has no icons, Start search doesn't respond, and
File Explorer tends to crash (a black flash, then nothing). Ignore all of it —
you work entirely from a command prompt. Open one through **Task Manager**
(<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Esc</kbd> → *File → Run new task* → `cmd`,
ticking *Create this task with administrative privileges*).

## After the install

You're now at the audit-mode desktop, working from a `cmd` prompt.

1. **Install Guest Additions** so the shared folder and clipboard work. Choose
   *Devices → Insert Guest Additions CD image* from the VirtualBox menu, then run
   the installer straight from your prompt — audit mode's flaky Explorer makes
   the CD autorun unreliable, so don't wait for it:

   ```cmd
   D:\VBoxWindowsAdditions.exe
   ```

   Click through it and reboot (back into audit mode).

2. **Snapshot the clean state** so you can reset to it between test runs instead
   of reinstalling:

   ```bash
   VBoxManage snapshot moor-windows-test take clean-install \
       --description "Audit-mode desktop + Guest Additions"
   ```

## Getting moor.exe into the VM

`scripts/windows-vm.sh` already shared the repo's `releases` folder into the VM,
so binaries you build on the host show up inside the guest on an auto-mounted
drive, or at `\\VBOXSVR\releases`. The share is read-only, which is all moor
needs — you run binaries straight off it, no copying.

Build what you want to test on the host:

```bash
scripts/windows-build.sh
```

This cross-compiles two Windows binaries into `releases/` under stable names:
`moor.exe` from the current branch and `moor-master.exe` from master. Comparing a
branch against master is the usual reason to test on Windows in the first place;
if you only care about one tree, `GOOS=windows GOARCH=amd64 ./build.sh` builds
just that under its version-stamped name.

Then, **once per VM**, put the share on the guest's `PATH` from an admin `cmd`:

```cmd
setx PATH "%PATH%;\\VBOXSVR\releases"
```

`setx` only affects *new* command prompts, so close that `cmd` and open a fresh
one (Task Manager → *File → Run new task* → `cmd`) before the change takes
effect. `where moor` should then print the path on the share. From then on you
just type:

```cmd
moor C:\Windows\System32\drivers\etc\hosts
dir /s C:\Windows | moor
moor-master C:\Windows\System32\drivers\etc\hosts
```

Because `PATH` points at the share, every host rebuild is picked up
automatically: rerun `scripts/windows-build.sh` on the host and the next `moor`
in the guest is the new build, with nothing to do guest-side.

## Per-session summary

Once the VM exists and the shared folder is wired, each testing session is just:

```bash
scripts/windows-build.sh   # rebuild moor.exe (branch) + moor-master.exe (master)
scripts/windows-vm.sh      # start the VM (or reset to the snapshot first)
```

## Host CPU while it's running

A running Windows 11 guest is never truly idle. Even with Task Manager *inside*
the guest reporting a few percent, the `VirtualBoxVM` process on the host can sit
at tens of percent of one CPU core. That's expected, not a runaway process:
Windows' background services (Defender, search, update) keep the guest a little
busy at all times, and virtualization amplifies each timer tick and privileged
instruction into host overhead. The VM is already tuned for this — Hyper-V
paravirtualization and nested paging are on — so there's nothing to fix.

The cheap fix is simply not to leave it running. This is a throwaway VM, so save
its state or power it off between test sessions; `scripts/windows-vm.sh` brings it
back in seconds.
