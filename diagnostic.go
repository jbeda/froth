package froth

// A Diagnostic is something froth detected and should not decide about: a
// message cycle, an intent nobody claimed, a surface that panicked. Each is
// nearly always a bug, and none of them is froth's to escalate — hence one hook
// rather than a policy.
//
// Kind is what code switches on; Detail is prose for whoever reads it. froth
// constructs every Diagnostic, so adding a field breaks no caller.
type Diagnostic struct {
	Kind   DiagnosticKind
	Detail string
}

// String renders a diagnostic as one line, which is what most hooks want.
func (d Diagnostic) String() string { return "froth: " + string(d.Kind) + ": " + d.Detail }

// A DiagnosticKind names what froth detected.
type DiagnosticKind string

// DiagIterationCap is reported when a Deliver/Apply cascade does not settle
// within the iteration cap. froth abandons the cascade rather than continuing,
// so the frame that follows is drawn from whatever state the cascade reached.
const DiagIterationCap DiagnosticKind = "iteration cap"

// DiagInvalidFocus is reported when focus is asked to move to a handle that is
// not in the tree. Focus does not move, and the surface that held it keeps it.
const DiagInvalidFocus DiagnosticKind = "invalid focus"

// report records a diagnostic for the hook. It does not call the hook: a hook is
// application code, and calling it mid-phase would hand it a froth that panics
// on the first thing it looked at. flush delivers them once the phase is over.
func (f *Froth) report(d Diagnostic) { f.diags = append(f.diags, d) }

// flushRounds bounds how many times one flush pass may hand the hook a fresh
// batch. A hook is application code and may drive froth, so a hook that
// provokes the very diagnostic it is handling would otherwise never return.
const flushRounds = 100

// flush hands the recorded diagnostics to the hook, with no phase in progress.
//
// The queue is taken rather than ranged over, so a diagnostic the hook causes
// goes out in the next round rather than in the middle of this one. A hook that
// drives froth re-enters flush from there; the nested call returns at once and
// leaves its batch to the pass already running, which is what keeps the hook one
// call deep however far into froth it reaches.
//
// Running out of rounds drops what is left. There is nothing better to do with
// it: the only way to report a diagnostic is the hook that is causing them.
func (f *Froth) flush() {
	if f.flushing {
		return
	}
	if f.onDiagnostic == nil {
		f.diags = nil
		return
	}

	f.flushing = true
	defer func() { f.flushing = false }()

	for round := 0; len(f.diags) > 0; round++ {
		if round == flushRounds {
			f.diags = nil
			return
		}
		diags := f.diags
		f.diags = nil
		for _, d := range diags {
			f.onDiagnostic(d)
		}
	}
}
