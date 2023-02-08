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
		if len(c.Attributes) > 0 {

			var cType string
			var cValue string

			switch c.Attributes[0] {
			case color.FgGreen:
				cType = "fg"
				cValue = "green"
			case color.FgMagenta:
				cType = "fg"
				cValue = "magenta"
			case color.FgRed:
				cValue = "red"
			case color.FgYellow:
				cType = "fg"
				cValue = "red"
			}
			if cType != "" {
				return fmt.Sprintf("[%s](%s:%s)", s, cType, cValue)
			}
		}
		return s
	default:
		return color.New(c.Attributes...).SprintfFunc()(s)
	}
}
