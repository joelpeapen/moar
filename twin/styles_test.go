package twin

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestHyperlinkToNormal(t *testing.T) {
	url := "http://example.com"

	style := StyleDefault.WithHyperlink(&url)
	assert.Equal(t,
		strings.ReplaceAll(StyleDefault.RenderUpdateFrom(style, ColorCount16), "\x1b", "ESC"),
		"ESC]8;;ESC\\")
}

func TestHyperlinkTransitions(t *testing.T) {
	url1 := "file:///Users/johan/src/riff/src/refiner.rs"
	url2 := "file:///Users/johan/src/riff/src/other.rs"

	// No link -> link
	styleNoLink := StyleDefault
	styleWithLink := StyleDefault.WithHyperlink(&url1)
	output := styleWithLink.RenderUpdateFrom(styleNoLink, ColorCount16)
	assert.Equal(t, strings.ReplaceAll(output, "\x1b", "ESC"), "ESC]8;;"+url1+"ESC\\")

	// Link -> different link
	styleWithLink2 := StyleDefault.WithHyperlink(&url2)
	output = styleWithLink2.RenderUpdateFrom(styleWithLink, ColorCount16)
	assert.Equal(t, strings.ReplaceAll(output, "\x1b", "ESC"), "ESC]8;;"+url2+"ESC\\")

	// Link -> no link
	output = styleNoLink.RenderUpdateFrom(styleWithLink, ColorCount16)
	assert.Equal(t, strings.ReplaceAll(output, "\x1b", "ESC"), "ESC]8;;ESC\\")

	// Link -> same link (no output)
	output = styleWithLink.RenderUpdateFrom(styleWithLink, ColorCount16)
	assert.Equal(t, output, "")
}

func TestBoldNoLinkToBoldLink(t *testing.T) {
	url := "file:///Users/johan/src/riff/src/refiner.rs"
	boldNoLink := StyleDefault.WithAttr(AttrBold)
	boldWithLink := boldNoLink.WithHyperlink(&url)

	output := boldWithLink.RenderUpdateFrom(boldNoLink, ColorCount16)
	assert.Equal(t, strings.ReplaceAll(output, "\x1b", "ESC"), "ESC]8;;"+url+"ESC\\")
}

func TestRenderUpdateFromAllSupportedTextAttributes(t *testing.T) {
	testCases := []struct {
		name    string
		onCode  string
		offCode string
		attr    AttrMask
	}{
		{name: "bold", onCode: "1", offCode: "22", attr: AttrBold},
		{name: "dim", onCode: "2", offCode: "22", attr: AttrDim},
		{name: "italic", onCode: "3", offCode: "23", attr: AttrItalic},
		{name: "underline", onCode: "4", offCode: "24", attr: AttrUnderline},
		{name: "blink", onCode: "5", offCode: "25", attr: AttrBlink},
		{name: "reverse", onCode: "7", offCode: "27", attr: AttrReverse},
		{name: "hidden", onCode: "8", offCode: "28", attr: AttrHidden},
		{name: "strikethrough", onCode: "9", offCode: "29", attr: AttrStrikeThrough},
	}

	baseStyle := StyleDefault.WithForeground(NewColor16(1))

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			enabledStyle := baseStyle.WithAttr(tc.attr)
			onTransition := strings.ReplaceAll(enabledStyle.RenderUpdateFrom(baseStyle, ColorCount16), "\x1b", "ESC")
			assert.Equal(t, onTransition, "ESC["+tc.onCode+"m")

			disabledStyle := enabledStyle.WithoutAttr(tc.attr)
			offTransition := strings.ReplaceAll(disabledStyle.RenderUpdateFrom(enabledStyle, ColorCount16), "\x1b", "ESC")
			assert.Equal(t, offTransition, "ESC["+tc.offCode+"m")
		})
	}
}

func TestStyleStringAllSupportedTextAttributes(t *testing.T) {
	testCases := []struct {
		name        string
		attr        AttrMask
		expectedSub string
	}{
		{name: "bold", attr: AttrBold, expectedSub: "bold"},
		{name: "dim", attr: AttrDim, expectedSub: "dim"},
		{name: "italic", attr: AttrItalic, expectedSub: "italic"},
		{name: "underline", attr: AttrUnderline, expectedSub: "underlined"},
		{name: "blink", attr: AttrBlink, expectedSub: "blinking"},
		{name: "reverse", attr: AttrReverse, expectedSub: "reverse"},
		{name: "hidden", attr: AttrHidden, expectedSub: "hidden"},
		{name: "strikethrough", attr: AttrStrikeThrough, expectedSub: "strikethrough"},
	}

	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			styleString := StyleDefault.WithAttr(tc.attr).String()
			assert.Assert(t, strings.Contains(styleString, tc.expectedSub), styleString)
		})
	}
}
