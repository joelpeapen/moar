package linemetadata

// ScreenLines represents a vertical count of physical rows on the terminal
// screen.
//
// This is explicitly distinct from Index and Number, which represent logical
// lines from the input stream. A single input line (Index) can wrap into
// multiple ScreenLines if it contains more characters than the terminal is
// wide.
//
// CRITICAL RULE: Never cast ScreenLines to an int just to add or subtract it
// from a logical Index or Number. Because of line wrapping, one ScreenLine may
// not equal one Index. Mixing unit types can lead to panics!
//
// It can represent an absolute measure (like terminal height) or a relative
// offset (which can be negative, such as a scroll delta).
type ScreenLines int
