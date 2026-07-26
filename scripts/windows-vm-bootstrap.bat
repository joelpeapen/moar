@echo off
rem One-time in-guest setup for the moor Windows test VM. Copied into the shared
rem .windows-vm-io directory by scripts/windows-vm.sh, so run it in the guest as:
rem
rem     \\VBOXSVR\windows-vm-io\bootstrap.bat
rem
rem See WINDOWS-VM.md on the host for what this VM is and why.

setlocal
set FAILED=

rem Every step below needs administrative rights, and audit mode makes it easy to
rem open a prompt without them.
net session >nul 2>&1
if errorlevel 1 (
    echo Needs an elevated prompt: Task Manager, File, Run new task, cmd, with 1>&2
    echo "Create this task with administrative privileges" ticked. 1>&2
    exit /b 1
)

echo === Switching the keyboard layout to US ===
rem Injected keystrokes are US scancodes, re-mapped through whatever layout the
rem guest has. See "Driving the guest from the host" in WINDOWS-VM.md.
rem
rem Set-WinUserLanguageList replaces the whole list, so the layout picked during
rem install goes away rather than lingering as a second one you can Alt+Shift
rem into by accident. -ErrorAction Stop is what makes powershell.exe exit
rem non-zero on failure; without it a failed cmdlet still exits 0.
powershell -NoProfile -Command "Set-WinUserLanguageList -LanguageList en-US -Force -ErrorAction Stop"
if errorlevel 1 set FAILED=1

echo.
echo === Putting the shared folder on the PATH ===
rem Checking the stored value rather than %PATH% keeps this idempotent: %PATH% in
rem this window predates any earlier run, so it would append a duplicate.
reg query HKCU\Environment /v Path 2>nul | findstr /i /c:"VBOXSVR\windows-vm-io" >nul
if errorlevel 1 (
    rem This flattens the system PATH into the user's, which is untidy but
    rem harmless on a throwaway VM, and keeps the command short enough to read.
    rem setx truncates above 1024 characters, which a bare LTSC install is
    rem nowhere near.
    setx PATH "%PATH%;\\VBOXSVR\windows-vm-io"
    if errorlevel 1 set FAILED=1
) else (
    echo Already on the PATH.
)

echo.
echo === Making runcmd work in any prompt ===
rem scripts/windows-vm-run.sh types "runcmd" at whatever prompt has focus, so it
rem has to resolve even in a prompt older than the PATH change above — including
rem the one you're running this from. C:\Windows is always on the PATH, so a shim
rem there is found regardless.
> C:\Windows\runcmd.bat echo @call \\VBOXSVR\windows-vm-io\runcmd.bat %%*
if errorlevel 1 set FAILED=1

echo.
echo === Opening an elevated prompt at every logon ===
rem Audit mode has no usable taskbar or Start menu, and the evaluation licence
rem forces a reboot every hour, so getting a prompt by hand gets old fast.
rem
rem A scheduled task with an "at logon" trigger is the tidier-looking mechanism,
rem but audit mode's automatic logon raises no logon event for Task Scheduler, so
rem the task sits Ready and never runs. A Run key does fire, about a minute after
rem the desktop appears, and it's elevated because audit mode logs you in as the
rem built-in Administrator, whose token isn't UAC-filtered by default.
reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run" /v moorprompt /t REG_SZ /d "cmd.exe /k" /f
if errorlevel 1 set FAILED=1

rem Clean up after earlier versions of this script, which used the task.
schtasks /delete /tn moorprompt /f >nul 2>&1

echo.
echo === Clearing the Administrator password ===
rem Audit mode's automatic logon only works while this account has no password.
net user Administrator ""
if errorlevel 1 set FAILED=1

echo.
echo === Evaluation licence status ===
rem Expect "Notification" and "grace time expired": audit mode never arms the
rem 90-day evaluation clock, so the guest shuts itself down hourly. This is
rem informational, not a failure. See "The hourly shutdown" in WINDOWS-VM.md.
cscript //nologo C:\Windows\System32\slmgr.vbs /dlv | findstr /i "status reason rearm"

echo.
if defined FAILED (
    echo Some steps failed, see above. 1>&2
    exit /b 1
)

echo Done. Close this window and open a fresh prompt for the PATH change to take
echo effect, then check that "where moor" finds it on the shared folder.
