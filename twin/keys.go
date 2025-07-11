package twin

type KeyCode uint16

const (
	KeyEscape KeyCode = iota
	KeyEnter

	KeyBackspace
	KeyDelete

	KeyUp
	KeyDown
	KeyRight
	KeyLeft

	KeyAltUp
	KeyAltDown
	KeyAltRight
	KeyAltLeft

	RuneKeyAltRight
	RuneKeyAltLeft

	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown

	KeyF1
)

// Map incoming escape keystrokes to keycodes, used in consumeEncodedEvent() in
// screen.go.
//
// NOTE: If you put a single ESC character in here ('\x1b') it will be consumed
// by itself rather than as part of the sequence it belongs to, and parsing of
// all special sequences starting with ESC will break down.
//
// FIXME: Write a test preventing that from happening.
var escapeSequenceToKeyCode = map[string]KeyCode{
	// NOTE: Please keep this list in the same order as the KeyCode const()
	// section above.

	// KeyEscape intentionally left out because it's too short, see comment
	// above.

	// KeyEnter intentionally left out because it's too short, see comment
	// above.

	"\x7f":    KeyBackspace,
	"\x1b[3~": KeyDelete,

	"\x1b[A": KeyUp,
	"\x1b[B": KeyDown,
	"\x1b[C": KeyRight,
	"\x1b[D": KeyLeft,

	// Ref: https://github.com/walles/moar/issues/138#issuecomment-1579199274
	"\x1bOA": KeyUp,
	"\x1bOB": KeyDown,
	"\x1bOC": KeyRight,
	"\x1bOD": KeyLeft,

	"\x1bl": RuneKeyAltRight, // Alt + l
	"\x1bh": RuneKeyAltLeft,  // Alt + h

	"\x1b[1;3A": KeyAltUp,    // Alt + up arrow
	"\x1b[1;3B": KeyAltDown,  // Alt + down arrow
	"\x1b[1;3C": KeyAltRight, // Alt + right arrow
	"\x1b[1;3D": KeyAltLeft,  // Alt + left arrow

	"\x1b[H":  KeyHome,
	"\x1b[F":  KeyEnd,
	"\x1b[1~": KeyHome,
	"\x1b[4~": KeyEnd,
	"\x1b[5~": KeyPgUp,
	"\x1b[6~": KeyPgDown,

	"\x1bOP": KeyF1,
}
