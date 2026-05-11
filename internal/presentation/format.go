package presentation

import "io"

type Format string

const (
	FormatHuman Format = "human"
	FormatJSON  Format = "json"
)

type Options struct {
	Format        Format
	Color         bool
	TerminalWidth int
}

type Presenter struct {
	opts Options
}

func New(opts Options) Presenter {
	if opts.Format == "" {
		opts.Format = FormatHuman
	}
	return Presenter{opts: opts}
}

func (p Presenter) Present(w io.Writer, value any) error {
	if p.opts.Format == FormatJSON {
		return writeJSON(w, value)
	}
	return p.presentHuman(w, value)
}

func (p Presenter) PresentError(w io.Writer, err error) error {
	if p.opts.Format == FormatJSON {
		return writeJSON(w, ErrorEnvelopeFrom(err))
	}
	return writeHumanError(w, err)
}
