package jsondiffprinter

type Option func(*Formatter)

func WithColor(enabled bool) Option {
	return func(f *Formatter) {
		f.c.Disable = !enabled
	}
}

func WithIndentation(indentation string) Option {
	return func(f *Formatter) {
		f.indentation = indentation
	}
}

func WithIndentedDiffMarkers(indentedDiffMarkers bool) Option {
	return func(f *Formatter) {
		f.indentedDiffMarkers = indentedDiffMarkers
	}
}

func WithCommas(commas bool) Option {
	return func(f *Formatter) {
		f.commas = commas
	}
}

func WithHideUnchanged(hideUnchanged bool) Option {
	return func(f *Formatter) {
		f.hideUnchanged = hideUnchanged
	}
}

func WithJSONinJSONCompare(jsonInJSONComparer Comparer) Option {
	return func(f *Formatter) {
		f.jsonInJSONComparer = jsonInJSONComparer
	}
}
