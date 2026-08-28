package cli

import (
	"fmt"
	"io"

	"github.com/focalspan/focalspan/internal/app"
)

type indexProgressReporter struct {
	writer        io.Writer
	label         string
	phase         string
	lastCompleted int
}

func newIndexProgressReporter(writer io.Writer, label string) app.IndexProgressFunc {
	reporter := &indexProgressReporter{writer: writer, label: label, lastCompleted: -1}
	return reporter.report
}

func (r *indexProgressReporter) report(event app.IndexProgress) {
	if r.phase != event.Phase {
		r.phase = event.Phase
		r.lastCompleted = -1
	}
	switch event.Phase {
	case app.IndexPhaseScanning:
		r.line("scanning repository...")
	case app.IndexPhaseChecking:
		r.count("checking", event)
	case app.IndexPhaseParsing:
		r.count("parsing", event)
	case app.IndexPhaseWriting:
		r.line("writing index...")
	case app.IndexPhaseComplete:
		r.line("complete")
	}
}

func (r *indexProgressReporter) count(phase string, event app.IndexProgress) {
	if event.Total > 0 {
		step := (event.Total + 99) / 100
		if event.Completed != 0 && event.Completed != event.Total && event.Completed-r.lastCompleted < step {
			return
		}
	}
	if event.Completed == r.lastCompleted {
		return
	}
	_, _ = fmt.Fprintf(r.writer, "%s: %s %d/%d files\n", r.label, phase, event.Completed, event.Total)
	r.lastCompleted = event.Completed
}

func (r *indexProgressReporter) line(message string) {
	_, _ = fmt.Fprintf(r.writer, "%s: %s\n", r.label, message)
}
