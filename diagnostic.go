package froth

import "fmt"

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
func (d Diagnostic) String() string {
	return fmt.Sprintf("froth: %s: %s", d.Kind, d.Detail)
}

// A DiagnosticKind names what froth detected.
type DiagnosticKind string

// DiagIterationCap is reported when a Deliver/Apply cascade does not settle
// within the iteration cap. froth abandons the cascade rather than continuing,
// so the frame that follows is drawn from whatever state the cascade reached.
const DiagIterationCap DiagnosticKind = "iteration cap"

// DiagInvalidFocus is reported when focus is asked to move to a handle that is
// not in the tree. Focus does not move, and the surface that held it keeps it.
const DiagInvalidFocus DiagnosticKind = "invalid focus"

// DiagDeadHandle is reported when a handle-scoped context is minted for a
// handle that names no live surface. The context is handed back rather than
// refused and nothing asked of it reaches a surface, so the surface that asked
// runs to the end of what it was doing rather than branching on whether its
// target is still there.
const DiagDeadHandle DiagnosticKind = "dead handle"

// DiagUntakenMessage is reported when a rising message climbs past every
// surface on its path and nothing calls StopRising. It is almost always a
// missing case or a surface parented somewhere unexpected, and it is the only
// thing that catches a forgotten StopRising, since Go cannot force the call.
//
// It is reported once per message type per update, however many messages of
// that type went untaken: what is missing is a case, and a case is a property
// of the type rather than of any one message.
const DiagUntakenMessage DiagnosticKind = "untaken message"

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
// drives froth re-enters this from there; the nested call returns at once and
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

// A frothPanic is a panic froth raised itself, as opposed to one that came out
// of a surface. Panic quarantine recovers surface panics and takes the surface
// out of the tree; it re-panics these, because a program that violates froth's
// contract has a bug that a diagnostic would bury.
//
// It is an error so that a panic carrying one prints its message rather than a
// struct dump, and it is unexported so nothing outside froth can raise one.
type frothPanic struct{ msg string }

func (p frothPanic) Error() string { return p.msg }

// bug raises a panic attributed to froth. Every panic froth raises deliberately
// goes through here.
func bug(format string, args ...any) {
	panic(frothPanic{fmt.Sprintf(format, args...)})
}

// DiagSurfacePanic is reported when a surface panics inside a froth callback.
// froth recovers, takes the surface out of the tree by the ordinary destroy
// path, and never calls it again — including its Unmount, which is code from
// the same surface that has already failed once.
const DiagSurfacePanic DiagnosticKind = "surface panic"

// DiagNoReconciler is reported when a surface asks to be reconciled and does
// not implement Reconciler. Nothing is queued and nothing is called: the
// request names a callback that does not exist, which is a missing method
// rather than a condition to handle.
const DiagNoReconciler DiagnosticKind = "no reconciler"

// DiagPostAfterShutdown is reported when a surface posts a message during the
// unmount walk Shutdown runs. Nothing will drain the queue, so the message is
// dropped: it is pointless rather than illegal, and saying so is better than
// refusing a call that reads as reasonable.
const DiagPostAfterShutdown DiagnosticKind = "post after shutdown"
