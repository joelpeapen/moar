package twin

type Event any

type EventRune struct {
	rune rune
}

type EventKeyCode struct {
	keyCode KeyCode
}

type MouseButtonMask uint16

const (
	MouseWheelUp MouseButtonMask = 1 << iota
	MouseWheelDown
	MouseWheelLeft
	MouseWheelRight
)

type EventMouse struct {
	buttons MouseButtonMask
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

func (eventRune *EventRune) Rune() rune {
	return eventRune.rune
}

func (eventKeyCode *EventKeyCode) KeyCode() KeyCode {
	return eventKeyCode.keyCode
}

func (eventMouse *EventMouse) Buttons() MouseButtonMask {
	return eventMouse.buttons
}

// The following NewEvent* functions are meant to be used by embedding applications
// to feed input into the pager.

func NewEventRune(r rune) EventRune {
	return EventRune{rune: r}
}

func NewEventKeyCode(keyCode KeyCode) EventKeyCode {
	return EventKeyCode{keyCode: keyCode}
}

func NewEventMouse(buttons MouseButtonMask) EventMouse {
	return EventMouse{buttons: buttons}
}
