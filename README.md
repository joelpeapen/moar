[![Linux CI](https://github.com/walles/moar/actions/workflows/linux-ci.yml/badge.svg?branch=master)](https://github.com/walles/moar/actions/workflows/linux-ci.yml?query=branch%3Amaster)
[![Windows CI](https://github.com/walles/moar/actions/workflows/windows-ci.yml/badge.svg?branch=master)](https://github.com/walles/moar/actions/workflows/windows-ci.yml?query=branch%3Amaster)

Moar is a pager. It reads and displays UTF-8 encoded text from files or
pipelines.

`moar` is designed to just do the right thing without any configuration:

![Moar displaying its own source code](screenshot.png)

The intention is that Moar should be trivial to get into if you have previously
been using [Less](http://www.greenwoodsoftware.com/less/). If you come from Less
and find Moar confusing or hard to migrate to, [please report
it](https://github.com/walles/moar/issues)!

Doing the right thing includes:

- **Syntax highlight** source code by default using
  [Chroma](https://github.com/alecthomas/chroma)
- **Search is incremental** / find-as-you-type just like in
  [Chrome](http://www.google.com/chrome) or
  [Emacs](http://www.gnu.org/software/emacs/)
- Search becomes case sensitive if you add any UPPER CASE characters
  to your search terms, just like in Emacs
- [Regexp](http://en.wikipedia.org/wiki/Regular_expression#Basic_concepts)
  search if your search string is a valid regexp
- Supports displaying ANSI color coded texts (like the output from
  `git diff` [| `riff`](https://github.com/walles/riff) for example)
- Supports UTF-8 input and output
- **Transparent decompression** when viewing [compressed text
  files](https://github.com/walles/moar/issues/97#issuecomment-1191415680)
  (`.gz`, `.bz2`, `.xz`, `.zst`, `.zstd`) or [streams](https://github.com/walles/moar/issues/261)
- The position in the file is always shown
- Supports **word wrapping** (on actual word boundaries) if requested using
  `--wrap` or by pressing <kbd>w</kbd>
- [**Follows output** as long as you are on the last line](https://github.com/walles/moar/issues/108#issuecomment-1331743242),
  just like `tail -f`
- Renders [terminal
  hyperlinks](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda)
  properly
- **Mouse Scrolling** works out of the box (but
  [look here for tradeoffs](https://github.com/walles/moar/blob/master/MOUSE.md))

[For compatibility reasons](https://github.com/walles/moar/issues/14), `moar`
uses the formats declared in these environment variables if present:

- `LESS_TERMCAP_md`: Man page <b>bold</b>
- `LESS_TERMCAP_us`: Man page <u>underline</u>
- `LESS_TERMCAP_so`: [Status bar and search hits](https://github.com/walles/moar/issues/114)

For configurability reasons, `moar` reads extra command line options from the
`MOAR` environment variable.

Moar is used as the default pager by:

- [`px` / `ptop`](https://github.com/walles/px), `ps` and `top` for human beings
- [`riff`](https://github.com/walles/riff), a diff filter highlighting which line parts have changed

# Installing

Run `./test.sh`

Pick a binary for your platform from the releases folder and you can copy to your $PATH.

And now you can just invoke `moar` from the prompt!

Try `moar --help` to see options.

# Configuring

Do `moar --help` for an up to date list of options.

Environment variable `MOAR` can be used to set default options.

For example:

```bash
export MOAR='--statusbar=bold --no-linenumbers'
```

## Setting `moar` as your default pager

Set it as your default pager by adding...

```bash
export PAGER=/usr/local/bin/moar
```

... to your `.bashrc`.

# Issues

Issues are tracked [here](https://github.com/walles/moar/issues), or
you can send questions to <johan.walles@gmail.com>.
