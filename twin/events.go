package twin

type Event any

// EventRune, EventKeyCode and EventMouse can be constructed directly by
// embedding applications to feed input into the pager.
//
// Ref: https://github.com/walles/moor/pull/456

type EventRune struct {
	Rune rune
}

type EventKeyCode struct {
	KeyCode KeyCode
}

type MouseButtonMask uint16

const (
	MouseWheelUp MouseButtonMask = 1 << iota
	MouseWheelDown
	MouseWheelLeft
	MouseWheelRight
)

type EventMouse struct {
	Buttons MouseButtonMask
}

// After you get this, query Screen.Size() to get the new size
type EventResize struct {
	// This interface intentionally left blank
}

// If we're unable to continue showing the screen, we'll send this event and
// drop out.
//
// Ref: https://github.com/walles/moor/issues/126
type EventExit struct {
	// This interface intentionally left blank
}
