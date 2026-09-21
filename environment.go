package froth

// A Key identifies one typed Environment value. Keys are compared by pointer;
// the name is used only to make a missing binding diagnostic actionable.
type Key[T any] struct {
	name       string
	def        T
	hasDefault bool
}

// NewKey makes an Environment key with no default. Reading it without a
// binding panics.
func NewKey[T any](name string) *Key[T] { return &Key[T]{name: name} }

// NewKeyDefault makes an Environment key that resolves def when no surface in
// the reader's ancestry binds it.
func NewKeyDefault[T any](name string, def T) *Key[T] {
	return &Key[T]{name: name, def: def, hasDefault: true}
}

func (k *Key[T]) defaultValue() (T, bool) { return k.def, k.hasDefault }

// Env reads k through a handle-scoped context. It is useful to helpers that
// accept HandleReader and need the same typed read in more than one phase.
func Env[T any](r HandleReader, k *Key[T]) T {
	checkEnvKey("Env", k)
	if v, ok := r.lookupEnv(k); ok {
		return v.(T)
	}
	return missingEnv(k)
}

func envAt[T any](t phaseToken, h Handle, k *Key[T]) T {
	checkEnvKey("Env", k)
	t.check()
	if v, ok := t.f.lookupEnv(h, k); ok {
		return v.(T)
	}
	return missingEnv(k)
}

func missingEnv[T any](k *Key[T]) T {
	if def, ok := k.defaultValue(); ok {
		return def
	}
	bug("froth: Environment key %q has no binding and no default", k.name)
	var zero T
	return zero
}

func checkEnvKey[T any](operation string, k *Key[T]) {
	if k == nil {
		bug("froth: %s called with a nil Environment key", operation)
	}
}

func (f *Froth) lookupEnv(h Handle, key any) (any, bool) {
	for h != (Handle{}) {
		n := f.reg.get(h)
		if n == nil {
			return nil, false
		}
		if v, ok := n.envOwn[key]; ok {
			return v, true
		}
		if v, ok := n.envGiven[key]; ok {
			return v, true
		}
		h = n.parent
	}
	return nil, false
}

func setEnv[T any](t phaseToken, h Handle, own bool, k *Key[T], value T) {
	checkEnvKey("SetEnv", k)
	t.check()
	n := t.f.reg.get(h)
	if n == nil {
		return
	}
	if own {
		if n.envOwn == nil {
			n.envOwn = make(map[any]any)
		}
		n.envOwn[k] = value
		return
	}
	if n.envGiven == nil {
		n.envGiven = make(map[any]any)
	}
	n.envGiven[k] = value
}
