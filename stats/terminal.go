package stats

import (
	"fmt"

	"github.com/fatih/color"
)

type TermStyle struct {
	Attributes []color.Attribute
}

var (
	Today        = TermStyle{[]color.Attribute{color.FgWhite, color.BgMagenta}}
	FirstOfMonth = TermStyle{[]color.Attribute{color.FgCyan}}
	ValueLow     = TermStyle{[]color.Attribute{}}
	ValueMiddle  = TermStyle{[]color.Attribute{color.FgGreen}}
	ValueHigh    = TermStyle{[]color.Attribute{color.FgYellow}}
	Empty        = TermStyle{[]color.Attribute{}}
	Message      = TermStyle{[]color.Attribute{color.FgGreen, color.BgBlack}}
	Error        = TermStyle{[]color.Attribute{color.FgRed}}
	Header       = TermStyle{[]color.Attribute{color.FgMagenta}}
)

func colorize(c TermStyle, s string, oType OutputType) string {
	switch oType {
	case Dashboard:
		if len(c.Attributes) == 0 {
			return s
		}
		var cValue string
		switch c.Attributes[0] {
		case color.FgGreen:
			cValue = "green"
		case color.FgMagenta:
			cValue = "magenta"
		case color.FgRed:
			cValue = "red"
		case color.FgYellow:
			cValue = "yellow"
		}
		if cValue != "" {
			return fmt.Sprintf("[%s](fg:%s)", s, cValue)
		}
		return s
	default:
		return color.New(c.Attributes...).SprintfFunc()(s)
	}
}
