package jsondiffprinter

import "fmt"

const (
	colorReset    = "\033[0m"
	colorRed      = "\033[31m"
	colorGreen    = "\033[32m"
	colorYellow   = "\033[33m"
	colorDarkGrey = "\033[90m"
)

type colorize struct {
	disable bool
}

func (c colorize) red(str string) string {
	return c.colorize(colorRed, str)
}

func (c colorize) green(str string) string {
	return c.colorize(colorGreen, str)
}

func (c colorize) yellow(str string) string {
	return c.colorize(colorYellow, str)
}

func (c colorize) darkGrey(str string) string {
	return c.colorize(colorDarkGrey, str)
}

func (c colorize) colorize(color, str string) string {
	if c.disable {
		return str
	}
	return fmt.Sprintf("%s%s%s", color, str, colorReset)
}
