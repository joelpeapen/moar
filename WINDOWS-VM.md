# Testing moor on Windows in a VM

Some moor behaviour can only be verified interactively on real Windows: how the
keyboard and terminal modes behave in `cmd.exe`, mouse handling, and anything
that differs from Unix. This is a walkthrough for setting up a throwaway Windows
VM in [VirtualBox](https://www.virtualbox.org/) on macOS or Linux to do that
testing.

Three scripts carry the parts that can be automated: `scripts/windows-vm.sh`
creates and starts the VM, `scripts/windows-vm-bootstrap.bat` does the in-guest
setup, and `scripts/windows-vm-run.sh` runs a command in the guest and prints its
output on the host. What's left is prose because it genuinely can't be scripted:
the Windows install is an interactive wizard that would need a lot of
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

When the installer asks for a keyboard layout, pick **US** — see [Driving the
guest from the host](#driving-the-guest-from-the-host). The bootstrap script below
switches it over anyway, so a wrong answer here costs nothing.

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
all you need to run moor in `cmd.exe`. You never have to finish OOBE or create a
user account.

It does have a price on evaluation media — see [The hourly
shutdown](#the-hourly-shutdown).

Audit mode is bare: the taskbar has no icons, Start search doesn't respond, and
File Explorer tends to crash (a black flash, then nothing). Ignore all of it —
you work entirely from a command prompt. Open one through **Task Manager**
(<kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Esc</kbd> → *File → Run new task* → `cmd`,
ticking *Create this task with administrative privileges*). You only need that
dance once; the bootstrap script below arranges for one to open by itself on every
boot.

That self-opening prompt lags the desktop by about a minute, so an empty desktop
right after boot is not a failure. Waiting it out is less work than opening one
by hand.

## The hourly shutdown

On evaluation media, an audit-mode guest powers itself off roughly once an hour,
and the desktop watermark reads "Windows License is expired" from the very first
boot. The ISO is fine and so is your install: OOBE's specialize pass is what
starts the 90-day evaluation clock, and audit mode skips OOBE, so the clock is
never armed. An expired *evaluation* edition forces a shutdown every hour of
uptime, and an unarmed one counts as expired.

To confirm rather than guess, from an admin prompt:

```cmd
cscript //nologo C:\Windows\System32\slmgr.vbs /dlv
```

On a guest installed minutes ago this reports `License Status: Notification`,
`Notification Reason: 0xC004F009 (grace time expired)` and a
`GracePeriodRemaining` of 0 — a grace period that was never started, not one
that ran out.

So the working rule is: **a cold boot buys about an hour**. The timer restarts on
every boot, so rebooting costs a minute and hands you a fresh hour. Half an hour
of uninterrupted testing is entirely practical; a two-hour session is not.

Snapshots interact with this in a way that will bite you. Restoring a
*saved-state* snapshot resumes the guest with its shutdown timer already almost
elapsed, so you get a few minutes and then a shutdown, over and over. Take
snapshots with the VM **powered off** instead — those cold-boot on restore and
give you the full hour.

Four things look like fixes and are not, so don't spend an evening on them:

* `slmgr /rearm` — resets a timer that audit mode never arms, so the status does
  not change. It also burns one of the (few) available rearms.
* Rolling the guest clock back with `VBoxManage modifyvm --biossystemtimeoffset`
  — the licence has no expiry date to escape; `EvaluationEndDate` is unset
  (reported as `1601-01-01`, the zero FILETIME).
* Installing a volume (GVLK) product key to move off the `Eval` SKU — refused
  with `0xC004F069`, "product SKU not found", because evaluation media doesn't
  contain the non-eval edition's licence files at all.
* Going back and finishing OOBE, even with a working network — `sysprep /oobe`
  re-enters OOBE, but 24H2 loops: *Hi there* → *"Why did my PC restart?"* →
  update → reboot → *Hi there*. Handing OOBE an `unattend.xml` in
  `C:\Windows\Panther` doesn't break the loop either.

The only real escape is to never enter audit mode: install with an
`Autounattend.xml` that completes OOBE unattended, which avoids the crash loop and
arms the clock properly. That's a bigger job than this walkthrough covers.

## After the install

You're now at the audit-mode desktop, working from a `cmd` prompt.

1. **Install Guest Additions** so the shared folder works. Choose
   *Devices → Insert Guest Additions CD image* from the VirtualBox menu, then run
   the installer straight from your prompt — audit mode's flaky Explorer makes
   the CD autorun unreliable, so don't wait for it:

   ```cmd
   D:\VBoxWindowsAdditions.exe
   ```

   Click through it and reboot (back into audit mode).

2. **Run the bootstrap script**, which does the rest of the in-guest setup in one
   go — there's nothing to type but this one line:

   ```cmd
   \\VBOXSVR\windows-vm-io\bootstrap.bat
   ```

   It switches the keyboard layout to US so injected keystrokes aren't re-mapped,
   puts the shared folder on the `PATH`, arranges for an elevated `cmd` to open
   itself on every boot (because you'll be rebooting hourly, and the Task Manager
   dance gets old fast), drops a `runcmd` shim in `C:\Windows` so
   `scripts/windows-vm-run.sh` works in any prompt, clears the Administrator
   password, and prints the licence status so you know what you're dealing with.

   That password has to stay **blank**: audit mode's automatic logon depends on
   it, so setting one lands you at a logon screen on every boot instead of the
   desktop. And the layout change and `setx` only affect *new* prompts, so close
   this `cmd` and let the next boot hand you a fresh one.

3. **Snapshot the clean state** so you can reset to it between test runs instead
   of reinstalling. Do this last, so the snapshot captures Guest Additions and the
   bootstrapped setup rather than losing them on every restore. Shut the guest down
   from its prompt and snapshot it powered off, so restoring gives you a cold boot
   and a full hour before [the hourly shutdown](#the-hourly-shutdown):

   ```cmd
   shutdown /s /t 0
   ```

   ```bash
   VBoxManage snapshot moor-windows-test take clean-install \
       --description "Audit-mode desktop, Guest Additions, bootstrapped"
   ```

## Getting moor.exe into the VM

`scripts/windows-vm.sh` shares one directory into the guest: `.windows-vm-io`,
which appears there as `\\VBOXSVR\windows-vm-io` and on an auto-mounted drive
letter. Everything crossing between host and guest goes through it — binaries,
batch files, command output — and the bootstrap script put it on the guest's
`PATH`. It's writable, which is what lets the guest hand results back; see
[`.windows-vm-io/README.md`](.windows-vm-io/README.md).

Build what you want to test on the host:

```bash
scripts/windows-build.sh
```

This cross-compiles two Windows binaries into `.windows-vm-io/` under stable
names: `moor.exe` from the current branch and `moor-master.exe` from master.
Comparing a branch against master is the usual reason to test on Windows in the
first place; if you only care about one tree,
`GOOS=windows GOARCH=amd64 ./build.sh` builds just that under its version-stamped
name.

`where moor` in the guest should print the path on the share. From then on you
just type:

```cmd
moor C:\Windows\System32\drivers\etc\hosts
dir /s C:\Windows | moor
moor-master C:\Windows\System32\drivers\etc\hosts
```

Because `PATH` points at the share, every host rebuild is picked up
automatically: rerun `scripts/windows-build.sh` on the host and the next `moor`
in the guest is the new build, with nothing to do guest-side.

## Driving the guest from the host

The actual testing is you at the VM window, but for setup and diagnostics it's
much faster to run things from a host shell and get the output back as text:

```bash
scripts/windows-vm-run.sh 'cscript //nologo C:\Windows\System32\slmgr.vbs /dlv'
```

That needs the VM running with a `cmd` prompt focused. Interactive programs are
not usable this way — moor itself included — because nothing reads their output
until they exit and there's no terminal to drive. For those you press keys at the
guest instead — at the VM window, or with the keystrokes and screenshots below.

The rest of this section is how it works, which is what you need when it doesn't.

**Screenshots** are the fallback for anything on screen that never lands in a
file, and for OOBE, where no prompt exists yet:

```bash
VBoxManage controlvm moor-windows-test screenshotpng /tmp/vm.png
```

**Keystrokes** go in with `keyboardputstring`, which types into whatever has focus
inside the guest; `keyboardputscancode` takes raw scancodes, `1c 9c` being Return:

```bash
VBoxManage controlvm moor-windows-test keyboardputstring 'runcmd'
VBoxManage controlvm moor-windows-test keyboardputscancode 1c 9c
```

`keyboardputstring` sends **US scancodes**, which the guest re-maps through its own
keyboard layout — which is why the bootstrap script puts the guest on a US one.
That's what makes `keyboardputstring 'set "K=:\|/@[]{}~$"'` arrive as exactly those
characters. Any other layout leaves letters and digits working while quietly
corrupting punctuation, so don't change it.

Keys that aren't characters go in as scancode pairs instead, make code then break
code, from **scan code set 1** — so a key not listed here is easy to look up. The
extended ones take an `e0` prefix on both codes:

| Key | Make | Break |
| --- | --- | --- |
| Return | `1c` | `9c` |
| Esc | `01` | `81` |
| Space | `39` | `b9` |
| Tab | `0f` | `8f` |
| Backspace | `0e` | `8e` |
| Left Ctrl | `1d` | `9d` |
| Left Shift | `2a` | `aa` |
| Up | `e0 48` | `e0 c8` |
| Down | `e0 50` | `e0 d0` |
| Left | `e0 4b` | `e0 cb` |
| Right | `e0 4d` | `e0 cd` |
| Page Up | `e0 49` | `e0 c9` |
| Page Down | `e0 51` | `e0 d1` |
| Home | `e0 47` | `e0 c7` |
| End | `e0 4f` | `e0 cf` |

Modifiers are held by sending their make code, then the key, then both break
codes — <kbd>Ctrl</kbd>+<kbd>C</kbd> is `1d 2e ae 9d`. Several pairs can go in one
call, so `keyboardputscancode e0 51 e0 d1` is a Page Down, and a screenshot is how
you see what it did.

**Files carry everything else.** The host writes the real command into
`.windows-vm-io/runcmd.bat` and the guest redirects stdout, stderr and the exit
code back into `out.txt` on the same share for the host to read. `runcmd` resolves
through a one-line shim in `C:\Windows`, which is on the `PATH` of every prompt
including ones older than bootstrap. The guest console gets the command printed
into it as well, so a session is followable from the VM window — via `type` of a
file rather than `echo`, which would let a `>` or `&` in the command take effect.
Use CRLF line endings, `.bat` files being what they are; `.gitattributes` pins
that for the ones under version control. A sentinel line written last is how the
host knows the guest has finished rather than guessing with a sleep.

Four approaches that look promising but don't work on this guest:

* `VBoxManage guestcontrol ... run` can't log in as the audit-mode
  Administrator. It fails with "The specified user account on the guest is
  restricted and can't be used to logon" (`ERROR_ACCOUNT_RESTRICTION`), and
  giving the account a real password with `net user` doesn't change that.
* Pasting through the shared clipboard. <kbd>Ctrl</kbd>+<kbd>V</kbd> in
  `cmd.exe` produces nothing, even with `Clipboard Mode: Bidirectional` and
  Guest Additions installed.
* A scheduled task with an at-logon trigger, as a way to get that prompt on every
  boot. Audit mode's automatic logon raises no logon event for Task Scheduler, so
  the task stays `Ready` with `Last Result: 267011` ("has not yet run") forever.
  A `Run` key works instead, and its startup delay is the price of that.
* `VBoxManage controlvm ... acpipowerbutton` to shut the guest down. Nothing
  happens; the Sysprep dialog sitting on the audit-mode desktop is the likely
  culprit. Use `shutdown /s /t 0` in the guest.

## Per-session summary

Once the VM exists and the shared folder is wired, each testing session is just:

```bash
scripts/windows-build.sh   # rebuild moor.exe (branch) + moor-master.exe (master)
scripts/windows-vm.sh      # start the VM (or reset to the snapshot first)
```

Each boot lands you at the audit-mode desktop, with an elevated `cmd` appearing
by itself about a minute later — give it that minute before assuming something
broke. When you're done, `shutdown /s /t 0` in that prompt powers the guest off
cleanly.

On evaluation media, budget for rebooting the guest every hour, and let
`windows-vm.sh` cold boot it rather than resuming saved state — see [The hourly
shutdown](#the-hourly-shutdown).

## Host CPU while it's running

A running Windows 11 guest is never truly idle. Even with Task Manager *inside*
the guest reporting a few percent, the `VirtualBoxVM` process on the host can sit
at tens of percent of one CPU core. That's expected, not a runaway process:
Windows' background services (Defender, search, update) keep the guest a little
busy at all times, and virtualization amplifies each timer tick and privileged
instruction into host overhead. The VM is already tuned for this — Hyper-V
paravirtualization and nested paging are on — so there's nothing to fix.

The cheap fix is simply not to leave it running. This is a throwaway VM, so power
it off with `shutdown /s /t 0` between test sessions — not save its state, which
resumes an almost-elapsed shutdown timer. `scripts/windows-vm.sh` boots it back in
seconds, with a usable prompt about a minute later.
