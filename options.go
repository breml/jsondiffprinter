package jsondiffprinter

type Option func(*Formatter)

// WithColor provides an option for the formatter to enable or disable
// the color full output.
func WithColor(enabled bool) Option {
	return func(f *Formatter) {
		f.c.disable = !enabled
	}
}

// WithIndentation provides an option for the formatter to set the indentation
// string to use when formatting the output.
func WithIndentation(indentation string) Option {
	return func(f *Formatter) {
		f.indentation = indentation
	}
}

// WithIndentedDiffMarkers provides an option for the formatter to enable or disable
// the indentation of the diff markers.
// If enabled, the diff markers will be indented to match the indentation of the JSON.
// If disabled, the diff markers will be aligned to the left.
func WithIndentedDiffMarkers(indentedDiffMarkers bool) Option {
	return func(f *Formatter) {
		f.indentedDiffMarkers = indentedDiffMarkers
	}
}

// WithCommas provides an option for the formatter to enable or disable
// the commas at the end of the JSON items.
func WithCommas(commas bool) Option {
	return func(f *Formatter) {
		f.commas = commas
	}
}

// WithHideUnchanged provides an option for the formatter to enable or disable
// the hiding of unchanged items.
// If enabled, unchanged items will not be printed. But instead a summary will
// be printed mentioning the number of unchanged items.
// If disabled, all items will be printed.
func WithHideUnchanged(hideUnchanged bool) Option {
	return func(f *Formatter) {
		f.hideUnchanged = hideUnchanged
	}
}

// WithJSONinJSONCompare provides an option for the formatter to set the
// comparer to use when comparing JSON in JSON.
// If not set, JSON in JSON diffing is disabled.
func WithJSONinJSONCompare(jsonInJSONComparer Comparer) Option {
	return func(f *Formatter) {
		f.jsonInJSONComparer = jsonInJSONComparer
	}
}
