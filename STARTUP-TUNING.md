# Startup tuning: get rid of the alt-screen blink

Working notes for [issue #425](https://github.com/walles/moor/issues/425).

**Delete this file when all steps are done.**

## The problem

With `--quit-if-one-screen`, moor enters the alternate screen (`ESC[?1049h`)
and immediately leaves it again (`ESC[?1049l`) when the input fits on one
screen. The terminal flashes. Common annoyance when moor is `$PAGER` for git
and jj (`git branch`, `git diff`, `git tag`, `jj status`, ...).

Measured today with a synthetic pty, 80x24, `printf 'hello\nworld\n' | moor
--quit-if-one-screen`:

| Case                                | Alt screen visible |
| ----------------------------------- | ------------------ |
| Terminal answers OSC 11 instantly   | 6.4 ms             |
| Terminal never answers OSC 11       | 56 ms              |

During that window moor paints a *full* screen, line numbers and reverse-video
status bar included, then a second delta redraw, then leaves. That is why it
reads as a blink rather than as a scroll.

## What makes a fix possible

Terminal size does not depend on the alternate screen. `UnixScreen.Size()` is
`term.GetSize(ttyOut.Fd())` where `ttyOut` is `os.Stdout`. So we can tell
whether the content fits without ever sending `ESC[?1049h`.

Also note that `TerminalBackground()` (`twin/screen.go:708`) has one shared
absolute deadline, 50 ms counted from when the query was sent. The first caller
may block; every later caller returns immediately, either with the color or
with nil. So there is no duplicate waiting to eliminate, only one wait to move
out of the way.

## What does not work

- **Never create the screen at all until we know the input is short.**
  Circular dependency: highlighting waits for the style, the style waits for
  the OSC 11 terminal background answer, and OSC 11 needs raw mode on the tty.
  Also no event loop means no keypress handling while we wait.
- **Reuse the `--no-clear-on-exit` path whenever `--quit-if-one-screen` is
  set.** Breaks the case where the content does not fit: we would scroll a
  screenful into the user's scrollback and then page inside it.

Note that highlighting genuinely matters for the fits-on-one-screen decision,
not just for colors: `ShouldFormat` reformats JSON, so a one-line input can
become a hundred lines.

## Steps

One branch per step, each merged into master separately.

- [x] **1. Do the OSC 11 query and the wait for its answer before entering the
      alternate screen.**
      In `NewScreen`, reorder to: start `mainLoop()`, send the query, call
      `TerminalBackground()`, and only then `enterAlternateScreenSession()`
      (`twin/screen.go:176`). Moving only the query is pointless: the 50 ms
      deadline starts ticking when the query is sent, and the blocking
      `TerminalBackground()` call would still happen with the alt screen
      already up. Includes making the query newline free:
      `fmt.Println("\x1b]11;?\x07")` at `twin/screen.go:191` must become a
      newline-free `writeLocked` write (hold `renderLock`), or every moor run
      scrolls the terminal one line once the query moves to the normal screen.
      Value on its own: takes the worst case (terminals that never answer OSC
      11) from ~56 ms of visible alt screen down to ~6 ms. Biggest cheap win.
      Cost: on terminals that do not answer OSC 11, a slow stream's first paint
      is delayed by up to 50 ms. Acceptable, and step 8 would introduce a grace
      period of the same order anyway.
      Branch: `query-background-before-alt-screen`. Measured afterwards: alt
      screen visible 0.8 ms when the terminal answers, 1.8 ms when it does not.
      Note that the blink is still clearly visible by eye, so the remaining
      steps are still needed.

- [x] **2. Split `redraw()` into render-cells and `Show()`.**
      Use the cells-only variant for the pre-exit redraw at
      `internal/pager.go:677`. Value on its own: removes a whole screen frame
      that is currently written to the terminal and then thrown away on every
      quit-if-one-screen exit. Prerequisite for step 5, otherwise that final
      redraw re-triggers the alt screen switch.
      Branch: `split-redraw`. Ended up going further: `ReprintAfterExit()` now
      renders the cells it prints from, so there is no render at all left on
      the way out of the main loop. That also removed a latent bug where a
      window resize between the last redraw and the reprint would blank the
      reprinted lines.

- [x] **3. A pty test harness, characterizing current behavior.**
      Land it with passing assertions: a long file does emit `ESC[?1049h`, and
      quit-if-one-screen output still lands on the normal screen. Value on its
      own: nothing in the repo can currently assert on escape sequences at all.
      Also the measurement tool for steps 1 and 5.
      Branch: `pty-test-harness`. Went with `creack/pty`, see the test notes
      below. What landed captures bytes, not timestamps, so it can tell you
      *whether* `ESC[?1049h` was written but not for how long the alternate
      screen was up. The numbers in the table above still come from throwaway
      measuring code.

- [x] **4. Make enter/leave alt screen idempotent, defer the enter to the first
      `Show()`.**
      An `alternateScreenActive` flag guarding
      `enterAlternateScreenSession()` (`twin/screen.go:276`) and
      `leaveAlternateScreenSession()` (`twin/screen.go:290`), a
      `GoInteractive()` method on the `Screen` interface with a no-op in
      `twin/fake-screen.go`, and dropping the enter from `NewScreen`
      (`twin/screen.go:176`). `Show()` activates implicitly so no code path can
      accidentally dump a frame on the normal screen; `ShowNLines()` does not,
      that is the `ReprintAfterExit()` path. Value on its own: the cursor is no
      longer hidden and mouse tracking is no longer on while moor sits at the
      shell prompt doing setup, and the unpaired-`ESC[?1049l` hazard in
      `Close()` and `PauseAndCall()` goes away. An unpaired `ESC[?1049l` is not
      harmless: DECRST 1049 also restores a saved cursor position, so it can
      teleport the cursor.
      Verify during this step that nothing else writes to the terminal between
      `NewScreen` and the first `Show()`.
      Branch: `defer-alt-screen-enter`. Verified: on the happy path nothing
      writes to the terminal in that window. `twin`'s only write path is
      `writeLocked()`, `cmd/moor/moor.go` writes to stderr only before
      `newScreen()` or after `Close()`, and logrus goes to a buffer. A *panic*
      in that window still prints on the normal screen, which after this step is
      strictly better than being stuck on the alternate one.
      Idempotency alone wasn't enough, two more flags were needed, both
      guarding against a concurrent `Show()` putting us back on the alternate
      screen: `closed`, because the signal handler `Close()`s while the pager
      goroutine is still running, and `paused`, for the `PauseAndCall()` window
      where the terminal belongs to an editor or to the shell after Ctrl-Z.
      `PauseAndCall()` now also restores what it found rather than
      unconditionally entering. That fixes a real old unpaired leave: Ctrl-C
      while the editor was running used to hit `PauseAndCall()`'s leave *and*
      `Close()`'s leave, teleporting the cursor. `Show()` bails out instead of
      painting when the alternate screen isn't ours.
      Skipped `GoInteractive()`: nothing calls it before step 8, and adding a
      method to the exported `Screen` interface is a breaking change for
      anybody implementing it. Step 8 can add it if it turns out to need it;
      step 5 did not.
      This step is byte-neutral: `redraw()` still runs before the
      quit-if-one-screen check, so the first `Show()` still enters the
      alternate screen just as early as the old `NewScreen` did.

- [x] **5. Evaluate quit-if-one-screen before the first redraw.**
      The check sat below the redraw, and the redraw is what enters the
      alternate screen, so every quit-if-one-screen exit painted a frame before
      deciding it was going to leave. Move it above. Reproduce with a failing
      pty test first: short quick input with `--quit-if-one-screen` must never
      emit `ESC[?1049h`.
      Branch: `quit-if-one-screen-before-redraw`. Measured 20 runs each, before
      then after, alternate screen entries: 20/20 → 0/20 for a `.txt` file
      argument, and the same for `hello world` piped in. This fixes the case
      issue #425 is about, `git branch | moor`, but not every case, see steps 7
      and 8.
      No grace deadline and no `GetLineCount()` early-out were needed for this
      much, and `GoInteractive()` stayed out of the `Screen` interface, so step
      4's decision to leave it there still stands.

- [x] **6. Reset the terminal style after printing on the normal screen.**
      `renderLine()` (`twin/screen.go:1026`) can end a line with a color still
      set, and the last line of a `ShowNLines()` render has nothing after it to
      clear that, so the style applies to whatever the shell prints next. Only
      lines narrower than the screen were safe, because their clear to end of
      line carries a reset with it.
      Not really part of the loop restructuring, and older than this whole
      effort: it hits `--no-clear-on-exit` just as hard, and would have been
      worth fixing if issue #425 had never been filed. It is here because this
      is where it was noticed. Note that the alternate screen leave, which the
      earlier draft of this plan credited with resetting the style, happens
      *before* the reprint and so never reset anything the reprint wrote.
      Branch: `quit-if-one-screen-before-redraw`, same branch as step 5, own
      commit. `showNLines()` now ends its non-fullscreen output with `ESC[m`.
      It went unnoticed for so long because the leftover background is usually
      the one the terminal itself reported in its OSC 11 answer. A full width
      colored line makes it obvious, which is what
      `TestReprintEndsWithStyleReset` pages.

- [x] **7. Close the highlighting race.**
      `moor --quit-if-one-screen foo.go` still blinks, measured 7/15 on a 20
      line `.go` file. `HighlightingDone` is only preset at construction when
      there is no lexer (`internal/reader/constructors.go:56-58`), which is why
      `.txt` files and piped plain text came out clean in step 5. With a lexer,
      highlighting blocks on `<-reader.highlightingStyle` until
      `cmd/moor/moor.go:602`, a few hundred microseconds before the main loop
      starts, and then loses the race about half the time. The margin is
      30-370 µs.
      Pipes are in scope too, not just file arguments: `NewFromStream`
      (`internal/reader/constructors.go:80-98`) doesn't preset
      `HighlightingDone` either. Piped plain text only wins the race because
      `highlightFromMemory` bails out early
      (`internal/reader/highlight.go:144-155`). Piped JSON, and anything
      `enry.GetLanguage()` classifies, race exactly like a `.go` file argument.
      This one needs no invented number, which is why it is not part of step 8:
      the decision is otherwise ready, so all that is missing is holding the
      first paint until highlighting lands. Don't block the main loop waiting
      for it. Skip the `p.redraw()` instead and fall through to
      `event := <-screen.Events()`: `HighlightingDone.Store(true)` is followed
      by a `MaybeDone` signal (`internal/reader/consumer.go:42-46`), the
      goroutine at `internal/pager.go:641-643` turns that into an event, and
      both channels are buffered so the wakeup cannot be dropped. Keys stay live
      and the reader needs no new API. Re-evaluate every lap rather than holding
      exactly one frame: `highlightFromMemory` signals `MoreLinesAdded` before
      `HighlightingDone`, so waking up with highlighting still pending is
      normal.
      The gate is `p.fitsOnOneScreen()` itself: hold only when the decision
      would otherwise have said yes and highlighting is the one thing left. It
      is already the inner test at `internal/pager.go:670`, so this is a swap,
      moving it out into the outer condition and leaving `HighlightingDone`
      inside. That adds no new predicate, where the `GetLineCount() > height`
      gate this step first called for would have inlined a copy of
      `fitsOnOneScreen()`'s own first lines (`internal/pager.go:819-821`),
      giving two places to drift apart. Extract the whole condition as
      `shouldHoldFirstPaint()`; step 8 reuses it and wants one place to extend.
      Highlighting completing is not actually guaranteed: a panic in the reader
      goroutine is only logged (`internal/reader/panicHandler.go:9-18`), leaving
      `HighlightingDone` false forever. So add a `sawUserInput` flag, set in the
      key/rune/mouse cases at `internal/pager.go:696-705`, and never hold once
      it is set. That bounds a panicked reader to a blank screen until the user
      touches something rather than a hang, and step 8 wants the same condition
      anyway. Worth a separate commit: have the deferred handler at
      `internal/reader/constructors.go:157-163` mark reading and highlighting
      done, which also fixes the forever-spinning spinner a panic leaves behind.
      `MOOR=--quit-if-one-screen` plus a source file is a mainstream invocation,
      so this is the more valuable of the two remaining steps.
      Note that `p.redraw()` is unconditional once the check declines, so
      nothing currently holds the first paint back. Both it and the check sit
      inside `if len(screen.Events()) == 0` (`internal/pager.go:656`), which is
      another reason the hold is a skipped redraw rather than a wait: a blocking
      wait would sit inside a block we may not even enter.
      A reload cannot un-set `HighlightingDone` under us during startup.
      `reloadFromFile` (`internal/reader/watcher.go:54-55`) is the only place
      that clears it, its only caller is the tailing loop, and `readStream`
      starts that loop only after highlighting has finished once
      (`internal/reader/consumer.go:50`) and it sleeps a second before its first
      poll (`internal/reader/watcher.go:303-307`).
      Branch: `hold-first-paint-for-highlighting`. Measured with the pty harness
      on a 20 line `.go` file, 15 runs each: 5/15 alternate screen entries
      before, 0/15 after.
      The predicate ended up named `canQuitIfOneScreen()` rather than
      `shouldHoldFirstPaint()`, because the quit branch needs the same answer
      and only the hold cares about `sawUserInput`. Holding is then a two line
      `holdPaint := canQuit && !p.sawUserInput` at the call site. Step 8 wants
      the hold without `canQuitIfOneScreen()`'s `ReadingDone` requirement, so it
      will have to split the predicate rather than just reuse it.
      Read `HighlightingDone` *before* counting the lines, never after.
      Highlighting replaces the lines and only then sets the flag, so a flag
      read first means the lines counted afterwards are the highlighted ones.
      The other order can quit on a pre-highlighting count that highlighting
      already invalidated, which with `--reformat` means a truncated file on
      screen and exit code 0.
      The reader hardening this step listed as optional turned out to be
      required, and landed here rather than being deferred: with the hold in
      place, a panic while highlighting means moor paints *nothing* until a
      keypress, which is worse than the spinning spinner it used to mean.

- [ ] **8. Grace deadline for slow producers.**
      Short input that dribbles in over longer than the accidental window
      described under "Known residue" still flashes. Hold the first paint until
      reading finishes, a key or mouse event arrives, or a deadline of
      ~50-100 ms expires. Split step 7's `canQuitIfOneScreen()` so the hold gets
      the same checks without its `ReadingDone` requirement, and large inputs
      still never wait: the `fitsOnOneScreen()` part releases the hold by itself
      as soon as the growing input outgrows a screen, and the deadline then never
      matters. `sawUserInput` from step 7 is the "or a key or mouse event
      arrives" half, already done.
      The numbers under "Known residue" are the input to picking the deadline.
      Deliberately parked: unlike every other step, this one requires somebody
      to choose a timeout, and it only buys the case where the producer is slow
      *and* the output is short.

Ordering constraint: step 5 depends on steps 2 and 4. Steps 7 and 8 depend on
step 5, and are independent of each other.

## Deferred

Found while working on the above, unrelated to the blink. Not scheduled, and
not a reason to keep this file around.

- **`pkg/moor` leaves the terminal wrecked if paging panics.**
  `pageFromReader()` (`pkg/moor/embed-api.go`) calls `screen.Close()`
  undeferred and with no `recover()`, so a panic in `StartPaging()` leaves the
  embedding process on the alternate screen in raw mode. `cmd/moor/moor.go`
  gets this right, in a deferred function that closes the screen, re-panics if
  there was a panic, and reprints otherwise. Note that the fix is not a plain
  `defer screen.Close()`: `ReprintAfterExit()` has to run after the close, so
  it takes the same shape as the one in `moor.go`. Don't end up with two
  `Close()` calls, it is not written to be idempotent.

- **`--quit-if-one-screen` can drop the last line when `DeInitFalseMargin` is
  0.** `fitsOnOneScreen()` accepts `GetLineCount() == screenHeight` while
  `renderLines()` caps at `screenHeight - 1`. Reachable with
  `--no-clear-on-exit-margin 0`, and quietly through `pkg/moor`, which sets
  `QuitIfOneScreen` but never `DeInitFalseMargin`.

- **`FakeScreen.RequestTerminalBackgroundColor()` is dead code.**
  `twin/fake-screen.go:104`. It is not part of the `Screen` interface, no
  `UnixScreen` counterpart exists, and nothing calls it. Deletable as is.

- **`go test -count=5 ./internal/` fails `TestSearchHighlight` 4 times out of
  5.** `internal/screenLines_test.go:126`. The package level style variables in
  `internal/styling.go:15-29` are set up by `styleUI()` and never reset, so a
  test that runs after one which styled the UI sees the styles the other test
  left behind. Only the second and later runs of a `-count=N` are affected, so
  CI never sees it, but it does mean `-count=N` is not a usable flakiness check
  for this package. Predates this effort, and the same globals are why the tests
  added for step 7 stop their pagers when they are done.

- **`FakeScreen.Events()` returns nil, with a `TODO` on it.**
  `twin/fake-screen.go:112-115`. `<-nil` blocks forever, so no test using a
  plain `FakeScreen` can run the pager main loop, which is a large hole given
  how many tests use it. Step 7 sidesteps it with a local screen double
  embedding `*twin.FakeScreen`, rather than fixing it, because handing
  `FakeScreen` a real channel changes behavior under every existing user at
  once: the goroutine at `internal/pager.go:641-643` does an unguarded
  `screen.Events() <- eventMaybeDone{}`, which parks forever on today's nil
  channel and would start filling a buffer instead. Worth doing properly if a
  second test wants the main loop.

## Known residue after all of this

Input that is short but dribbles in over longer than the grace period will
still flash. Unavoidable without waiting longer before showing anything.

There is already a grace period, but nobody designed it: it is whatever moor's
own startup costs, and it is inverted. Measured with the pty harness, two lines
piped with a gap between them, 10 runs per cell, alt screen entries:

| gap    | terminal answers OSC 11 | terminal never answers |
| ------ | ----------------------- | ---------------------- |
| 50 ms  | 0/10                    | 0/10                   |
| 70 ms  | 9/10                    | 0/10                   |
| 150 ms | 10/10                   | 10/10                  |

So the terminals that answer promptly, the good ones, get the *shorter* window,
somewhere between 50 and 70 ms here. The 50 ms `TerminalBackground()` timeout is
what pads the other column. Both numbers move with machine speed, and an earlier
sweep on a busier machine put the answering column's edge closer to 40 ms. An
explicit deadline would at least be a number somebody chose.

Step 7's `fitsOnOneScreen()` gate looks at un-highlighted content, and
highlighting can in one case make content *start* fitting: `--reformat` plus a
single long JSON line that expands into a screenful. That input fails the gate,
so it keeps its blink. `--reformat` is off by default
(`cmd/moor/moor.go:397`).

## Test notes

- `test.sh:104-105` runs `echo "  (success)" | ./moor --quit-if-one-screen` on
  a real TTY. That is the output-correctness contract to preserve.
- `internal/pager_test.go` calls `Quit()` before `StartPaging`, so the main
  loop body never runs there. Steps 5, 7 and 8 do not disturb those tests,
  which also means they will not catch regressions in them. It couldn't run the
  loop even without the `Quit()`: `twin.FakeScreen.Events()` returns nil
  (`twin/fake-screen.go:112-115`) and `<-nil` blocks forever. A test that wants
  the loop needs a screen double embedding `*twin.FakeScreen` with a real events
  channel, which also gives it somewhere to count `Show()` calls. Embed
  `*twin.FakeScreen`, pointer included, since its methods are all on pointer
  receivers.
- Many tests use `twin.NewFakeScreen`, so any new `Screen` interface method
  needs a no-op there.
- `cmd/moor/moor_test.go` injects a fake `newScreen` into `pagerFromArgs`,
  which is the hook for testing startup reordering.
- `cmd/moor/pty-harness_test.go` runs moor on a pseudo terminal and captures
  everything it writes, escape sequences included. `startMoor()` takes a
  `moorOptions`, whose `answerBackgroundQuery` field is the difference between
  the two rows in the table at the top of this file. The alternate screen
  assertions live in `cmd/moor/alt-screen_test.go`; step 3 landed two of them
  and step 5 replaced one, see below.
- Uses `creack/pty`, which has no ConPTY support, so both files are build
  tagged `!windows`. `test.sh`'s cross compilation step doesn't build test
  files, but `.github/workflows/windows-ci.yml` runs `go test ./...` on a real
  Windows machine, so the tag is load bearing.
- Wait for painted contents, never for a mode setting escape sequence. Moor
  enters the alternate screen with its first redraw, so a `q` sent on seeing
  `ESC[?1049h` gets handled before anything is drawn and the test then measures
  nothing. Mode settings say nothing about what is on screen.
- `moorOptions.extraEnv` is how you give moor an `$EDITOR`, which is the only
  way to exercise `PauseAndCall()` from a test.
- `moorOptions.stdin` pipes input into moor, which is what makes
  `stdinIsRedirected` true and `git branch | moor` testable. It took reshaping
  the harness around `pty.Open()` plus an explicit `SysProcAttr{Setsid: true,
  Setctty: true, Ctty: 1}`, because `pty.StartWithSize()` wires stdin, stdout
  and stderr to the same pty. `Ctty` is an fd number in the child, and with
  stdin taken by the pipe the pty is fd 1. Moor's keypresses don't come from the
  controlling terminal, `twin/screen-setup.go:147` dups stdout for those, but
  SIGWINCH does.
- Step 5's failing test lives in `cmd/moor/alt-screen_test.go` as
  `TestQuitIfOneScreenNeverEntersAlternateScreen`, over a file/pipe times
  background-query-answered/not matrix. It asserts neither `ESC[?1049h` nor
  `ESC[?1049l`, and `TestLongInputUsesAlternateScreen` covers the other
  direction for both files and pipes. `TestQuitIfOneScreenPrintsOnNormalScreen`
  guarded nothing those two don't and is gone.
- Step 6's test is `cmd/moor/reprint_test.go`, kept out of
  `alt-screen_test.go` because the contract is about what `ReprintAfterExit()`
  leaves behind, `--no-clear-on-exit` included, not about the alternate screen.
  It needs a line exactly `ptyCols` wide, so say `ptyCols` rather than 80.
- `go test ./cmd/moor/` served a cached pass after an edit to `twin/screen.go`,
  so use `-count=1` when you have just changed something these tests page
  through. The "(cached)" marker is the tell.
- Every pty test so far pages input chroma does not highlight, `.txt` files and
  unclassifiable piped text, which is the only reason they are not flaky. Any
  test of the startup path wanting the *highlighted* case needs a file argument
  with a real extension, or piped JSON. Such a test cannot look for a multi-word
  phrase in the captured output, chroma puts SGR sequences between the tokens;
  assert on a single identifier instead.
- Step 7's tests are `internal/quit-if-one-screen_test.go` for the contract and
  `TestQuitIfOneScreenNeverEntersAlternateScreenWhenHighlighted` in
  `cmd/moor/alt-screen_test.go` for the end to end net. The pty one caught the
  blink 4 times in 20 pre-step-7 subtest runs, so it is a net and not a
  reproducer; the deterministic reproducer is the pager level one.
- Size the input for a pty test of the highlighting race. Four lines of Go are
  highlighted before the pager even starts, so such a test passes with or without
  step 7. Twenty dense lines lose the race often enough to be worth having, and
  still fit on the harness' 24 line screen. Time-sensitive either way: it is
  measuring a margin of tens of microseconds.
- The pager main loop cannot be run with a plain `twin.FakeScreen`, see the
  `Events()` item under Deferred. `internal/quit-if-one-screen_test.go` has a
  screen double that can, counts `Show()` calls, and uses sends on an unbuffered
  event channel as handshakes with the main loop so that nothing has to sleep.
  Ask it whether it painted *before* handing it an event it reacts to: the answer
  is read once the pager is free to run on, so a later lap's paint lands in it.
