package froth

// A phase is the part of an update froth is in, and therefore what a surface is
// allowed to do. Each phase has its own context type, so most illegal
// operations are compile errors; the runtime check exists for what types cannot
// catch — a surface retaining a context and using it in a later phase.
type phase uint8

const (
	phaseIdle phase = iota
	phaseDeliver
	phaseApply
	phaseRender
)

func (p phase) String() string {
	switch p {
	case phaseDeliver:
		return "Deliver"
	case phaseApply:
		return "Apply"
	case phaseRender:
		return "Render"
	default:
		return "Idle"
	}
}

// enter switches froth into a phase and returns the token every context handed
// out during it carries. Nothing froth does re-enters, so being asked to start a
// phase during one means a surface called back into its own froth instance —
// which would invalidate the context that surface is still holding, and report
// itself as a retained context several frames from where the mistake is.
func (f *Froth) enter(p phase) phaseToken {
	if f.phase != phaseIdle {
		bug("froth: Update or View called re-entrantly, during %s; "+
			"nothing may call back into froth from a surface", f.phase)
	}
	f.phase = p
	f.epoch++
	return phaseToken{f: f, phase: p, epoch: f.epoch}
}

func (f *Froth) leave() { f.phase = phaseIdle }

// current is the token for the phase froth is in. It is for the internal code
// that has to hand a context onward but was not itself given one — the surface
// callbacks froth makes outside any phase, and the teardown walk they can start.
// A token is not constructible outside froth for the reason phaseToken.check
// states; this is froth minting its own, not a way around that.
func (f *Froth) current() phaseToken {
	return phaseToken{f: f, phase: f.phase, epoch: f.epoch}
}

// A phaseToken is embedded in every context. It identifies the exact phase run
// the context was handed out during, so a context outliving that run is
// detected on use rather than acting on stale state.
type phaseToken struct {
	f     *Froth
	phase phase
	epoch uint64
}

func (t phaseToken) check() {
	if t.f == nil {
		bug("froth: this context was not handed out by froth; a surface is " +
			"called with the context for the phase it is in, and one cannot be " +
			"constructed")
	}
	if t.f.phase != t.phase || t.f.epoch != t.epoch {
		bug("froth: %s context used outside the phase it was "+
			"handed out in; contexts must not be retained", t.phase)
	}
}
