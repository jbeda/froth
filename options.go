package froth

// An Option configures a froth instance at construction, for its lifetime.
type Option func(*options)

type options struct {
	diagnostic func(Diagnostic)
}

// OnDiagnostic registers the hook froth reports through. There is one hook for
// everything, and it is called with no phase in progress, so a hook may read the
// tree — or render it — without tripping the re-entrancy check.
func OnDiagnostic(fn func(Diagnostic)) Option {
	return func(o *options) { o.diagnostic = fn }
}
