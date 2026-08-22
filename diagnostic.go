package froth

// A Diagnostic is something froth detected and should not decide about: a
// message cycle, an intent nobody claimed, a surface that panicked. Each is
// nearly always a bug, and none of them is froth's to escalate — hence one hook
// rather than a policy.
//
// Kind is what code switches on; Detail is prose for whoever reads it. Fields
// are added as later milestones add kinds, and froth constructs every
// Diagnostic, so a new field breaks nobody.
type Diagnostic struct {
	Kind   DiagnosticKind
	Detail string
}

// String renders a diagnostic as one line, which is what most hooks want.
func (d Diagnostic) String() string { return "froth: " + string(d.Kind) + ": " + d.Detail }

// A DiagnosticKind names what froth detected. The set grows with the milestones:
// M3 adds quarantined panics and unclaimed intents, M5 dead-handle drops and
// producer type mismatches.
type DiagnosticKind string

// DiagIterationCap is reported when a Deliver/Apply cascade does not settle
// within the iteration cap. froth abandons the cascade rather than continuing,
// so the frame that follows is drawn from whatever state the cascade reached.
const DiagIterationCap DiagnosticKind = "iteration cap"

// report records a diagnostic for the hook. It does not call the hook: a hook is
// application code, and calling it mid-phase would hand it a froth that panics
// on the first thing it looked at. flush delivers them once the phase is over.
func (f *Froth) report(d Diagnostic) { f.diags = append(f.diags, d) }

// flush hands the recorded diagnostics to the hook, with no phase in progress.
// The queue is taken rather than ranged over, so a hook that drives froth and
// causes another diagnostic reports it on that pass rather than here.
func (f *Froth) flush() {
	for len(f.diags) > 0 {
		diags := f.diags
		f.diags = nil
		if f.onDiagnostic == nil {
			return
		}
		for _, d := range diags {
			f.onDiagnostic(d)
		}
	}
}
