package froth_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

type envMsg struct{}

type envSurface struct {
	children []*envSurface
	handles  []froth.Handle
	self     froth.Handle
	onMount  func(*froth.ApplyCtx, *envSurface)
	onUpdate func(*froth.UpdateCtx, *envSurface)
	onRender func(*froth.RenderCtx, *envSurface) string
}

func (s *envSurface) Render(rc *froth.RenderCtx, _ froth.Size) string {
	if s.onRender != nil {
		return s.onRender(rc, s)
	}
	return "."
}

func (s *envSurface) Update(uc *froth.UpdateCtx, _ tea.Msg) {
	if s.onUpdate != nil {
		s.onUpdate(uc, s)
	}
}

func (s *envSurface) Mount(ac *froth.ApplyCtx) {
	s.self = ac.Self().Handle()
	for _, child := range s.children {
		s.handles = append(s.handles, ac.Self().Create(child))
	}
	if s.onMount != nil {
		s.onMount(ac, s)
	}
}

func TestEnvironmentDefaultsMissingNilAndIdentity(t *testing.T) {
	defaulted := froth.NewKeyDefault("mode", "default")
	missing := froth.NewKey[string]("missing")
	one := froth.NewKey[string]("same")
	two := froth.NewKeyDefault("same", "other")
	root := &envSurface{}
	root.onMount = func(ac *froth.ApplyCtx, _ *envSurface) { ac.Self().SetEnv(one, "one") }
	root.onRender = func(rc *froth.RenderCtx, _ *envSurface) string {
		return rc.Env(defaulted) + "/" + rc.Env(two)
	}
	f := newSized(t, root, 20, 1)
	if got, want := f.View().Content, "default/other"; got != want {
		t.Errorf("Environment defaults and identity = %q, want %q", got, want)
	}

	root.onRender = func(rc *froth.RenderCtx, _ *envSurface) string { return rc.Env(missing) }
	mustPanic(t, "no binding and no default", func() { f.View() })

	nilRead := &envSurface{onUpdate: func(uc *froth.UpdateCtx, _ *envSurface) { uc.Env((*froth.Key[string])(nil)) }}
	mustPanic(t, "nil Environment key", func() { newSized(t, nilRead, 1, 1).Update(envMsg{}) })
	nilWrite := &envSurface{onMount: func(ac *froth.ApplyCtx, _ *envSurface) {
		ac.Self().SetEnv((*froth.Key[string])(nil), "x")
	}}
	mustPanic(t, "nil Environment key", func() { newSized(t, nilWrite, 1, 1) })
}

func TestEnvironmentInheritanceSlotsAndSubtreeIsolation(t *testing.T) {
	key := froth.NewKey[string]("theme")
	deep := &envSurface{onRender: func(rc *froth.RenderCtx, _ *envSurface) string { return rc.Env(key) }}
	left := &envSurface{children: []*envSurface{deep}, onMount: func(ac *froth.ApplyCtx, _ *envSurface) {
		ac.Self().SetEnv(key, "left-own")
	}, onRender: func(rc *froth.RenderCtx, _ *envSurface) string { return rc.Env(key) }}
	right := &envSurface{onRender: func(rc *froth.RenderCtx, _ *envSurface) string { return rc.Env(key) }}
	root := &envSurface{children: []*envSurface{left, right}, onMount: func(ac *froth.ApplyCtx, s *envSurface) {
		ac.Self().SetEnv(key, "root-own")
		ac.Child(s.handles[0]).SetEnv(key, "left-given")
		ac.Child(s.handles[1]).SetEnv(key, "right-given")
	}}
	f := newSized(t, root, 30, 1)
	if got := f.Composite(left.self).Content; got != "left-own" {
		t.Errorf("left sibling = %q, want left-own", got)
	}
	if got := f.Composite(right.self).Content; got != "right-given" {
		t.Errorf("right sibling = %q, want right-given", got)
	}
	if got := f.Composite(deep.self).Content; got != "left-own" {
		t.Errorf("deep descendant = %q, want left-own", got)
	}
}

func TestEnvironmentOwnSlotWinsRegardlessOfWriteOrder(t *testing.T) {
	key := froth.NewKey[string]("theme")
	child := &envSurface{onMount: func(ac *froth.ApplyCtx, _ *envSurface) {
		ac.Self().SetEnv(key, "own")
	}, onRender: func(rc *froth.RenderCtx, _ *envSurface) string { return rc.Env(key) }}
	root := &envSurface{children: []*envSurface{child}, onUpdate: func(uc *froth.UpdateCtx, s *envSurface) {
		uc.Child(s.handles[0]).SetEnv(key, "given-later")
	}}
	f := newSized(t, root, 10, 1)
	f.Update(envMsg{})
	if got, want := f.View().Content, "own"; got != want {
		t.Errorf("own Environment slot after later parent write = %q, want %q", got, want)
	}
}

func TestEnvironmentWritesAreImmediateAndCascadeDuringApply(t *testing.T) {
	key := froth.NewKey[string]("theme")
	var delivered string
	child := &envSurface{onMount: func(ac *froth.ApplyCtx, _ *envSurface) {
		if got := ac.Env(key); got != "apply" {
			t.Errorf("ApplyCtx.Env() = %q, want apply", got)
		}
	}, onUpdate: func(uc *froth.UpdateCtx, _ *envSurface) { delivered = uc.Env(key) }}
	root := &envSurface{children: []*envSurface{child}, onMount: func(ac *froth.ApplyCtx, _ *envSurface) {
		ac.Self().SetEnv(key, "apply")
	}, onUpdate: func(uc *froth.UpdateCtx, s *envSurface) {
		uc.Self().SetEnv(key, "deliver")
		uc.Child(s.handles[0]).Post(envMsg{})
	}}
	f := newSized(t, root, 10, 1)
	f.Update(envMsg{})
	if got, want := delivered, "deliver"; got != want {
		t.Errorf("child read after parent Deliver write = %q, want %q", got, want)
	}
}

func TestEnvironmentChildWritesAreImmediateInDeliver(t *testing.T) {
	key := froth.NewKey[string]("theme")
	var delivered string
	child := &envSurface{onUpdate: func(uc *froth.UpdateCtx, _ *envSurface) { delivered = uc.Env(key) }}
	root := &envSurface{children: []*envSurface{child}, onUpdate: func(uc *froth.UpdateCtx, s *envSurface) {
		uc.Child(s.handles[0]).SetEnv(key, "given")
		uc.Child(s.handles[0]).Post(envMsg{})
	}}
	f := newSized(t, root, 10, 1)

	f.Update(envMsg{})

	if got, want := delivered, "given"; got != want {
		t.Errorf("child read after parent Deliver write = %q, want %q", got, want)
	}
}

func TestEnvironmentReadsAllContextsAndHandleReader(t *testing.T) {
	key := froth.NewKey[string]("value")
	child := &envSurface{}
	var got []string
	root := &envSurface{children: []*envSurface{child}}
	root.onMount = func(ac *froth.ApplyCtx, s *envSurface) {
		ac.Self().SetEnv(key, "ok")
		got = append(got, ac.Env(key), ac.Self().Env(key), ac.Child(s.handles[0]).Env(key), ac.At(s.handles[0]).Env(key))
	}
	root.onUpdate = func(uc *froth.UpdateCtx, s *envSurface) {
		got = append(got, uc.Env(key), uc.Self().Env(key), uc.Child(s.handles[0]).Env(key), uc.At(s.handles[0]).Env(key), froth.Env(uc.At(s.handles[0]), key))
	}
	root.onRender = func(rc *froth.RenderCtx, s *envSurface) string {
		got = append(got, rc.Env(key), rc.Self().Env(key), rc.Child(s.handles[0]).Env(key), rc.At(s.handles[0]).Env(key))
		return "."
	}
	f := newSized(t, root, 1, 1)
	f.Update(envMsg{})
	f.View()
	if want := strings.Repeat("ok", len(got)); strings.Join(got, "") != want {
		t.Errorf("Environment reads = %v, want every reader to resolve ok", got)
	}
}

func TestEnvironmentDeadReaderUsesDefaultAndReports(t *testing.T) {
	key := froth.NewKeyDefault("theme", "default")
	child := &envSurface{}
	var got string
	root := &envSurface{children: []*envSurface{child}}
	root.onUpdate = func(uc *froth.UpdateCtx, s *envSurface) {
		if s.children[0].self == (froth.Handle{}) {
			return
		}
		if got == "" {
			uc.Child(s.children[0].self).Destroy()
			return
		}
	}
	f, diags := newWatched(t, root, 1, 1)
	f.Update(envMsg{})
	root.onUpdate = func(uc *froth.UpdateCtx, _ *envSurface) { got = uc.At(child.self).Env(key) }
	f.Update(envMsg{})
	if got != "default" {
		t.Errorf("dead reader default = %q, want default", got)
	}
	if !hasKind(*diags, froth.DiagDeadHandle) {
		t.Errorf("dead reader diagnostics = %v, want %q", *diags, froth.DiagDeadHandle)
	}

	missing := froth.NewKey[string]("missing")
	root.onUpdate = func(uc *froth.UpdateCtx, _ *envSurface) { uc.At(child.self).Env(missing) }
	mustPanic(t, "no binding and no default", func() { f.Update(envMsg{}) })
}

func TestEnvironmentChildAuthorityIsCheckedAtMint(t *testing.T) {
	key := froth.NewKey[string]("value")
	a, b := &envSurface{}, &envSurface{}
	a.onUpdate = func(uc *froth.UpdateCtx, _ *envSurface) { uc.Child(b.self).SetEnv(key, "no") }
	root := &envSurface{children: []*envSurface{a, b}, onUpdate: func(uc *froth.UpdateCtx, s *envSurface) {
		uc.Child(s.handles[0]).Post(envMsg{})
	}}
	f := newSized(t, root, 1, 1)
	mustPanic(t, "not an immediate child", func() { f.Update(envMsg{}) })
}

func mustPanic(t *testing.T, want string, call func()) {
	t.Helper()
	defer func() {
		if got := panicText(recover()); !strings.Contains(got, want) {
			t.Errorf("panic = %q, want it to contain %q", got, want)
		}
	}()
	call()
}
