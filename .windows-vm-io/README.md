# Windows VM I/O directory

This directory is how the host talks to the Windows test VM described in
[../WINDOWS-VM.md](../WINDOWS-VM.md). It is shared into the guest **writable**,
as `\\VBOXSVR\windows-vm-io`, which makes it the only place the guest can hand
data back to the host.

`scripts/windows-vm-run.sh` uses it in both directions:

* `runcmd.bat` — written by the host, run by the guest. It wraps whatever command
  you asked for.
* `out.txt` — written by the guest, read by the host. Holds the command's stdout,
  stderr and exit code.

Passing commands as files is how anything gets back out, keystrokes only going one
way; `scripts/windows-vm-run.sh` gives the full reasoning in its header comment.

Everything here except this README is gitignored: it's transient traffic, not
source. This README is committed so that the directory exists after a fresh
checkout — the shared folder is a permanent entry in the VM's configuration and
mounts at boot, and a missing host path makes it silently fail to appear in the
guest.
