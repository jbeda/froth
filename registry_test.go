package froth_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// parent assembles its children when it enters the tree. Each child is a box of
// its own size, so a child drawn later is visible on top of one drawn earlier.
type parent struct {
	self    froth.Handle
	kids    []froth.Surface
	handles []froth.Handle
	mounts  int
	body    string
}

func (p *parent) Render(rc *froth.RenderCtx, r froth.Rect) string { return p.body }

func (p *parent) Update(uc *froth.UpdateCtx, msg tea.Msg) {}

func (p *parent) Mount(ac *froth.ApplyCtx) {
	p.mounts++
	p.self = ac.Self()
	for _, k := range p.kids {
		p.handles = append(p.handles, ac.Create(k))
	}
}

// block renders a solid rectangle of one character, which makes overlap
// obvious: whatever is on top wins every cell it covers.
type block struct {
	ch   string
	w, h int
	self froth.Handle
	rect froth.Rect
}

func (b *block) Render(rc *froth.RenderCtx, r froth.Rect) string {
	b.rect = r
	rows := make([]string, b.h)
	for i := range rows {
		rows[i] = strings.Repeat(b.ch, b.w)
	}
	return strings.Join(rows, "\n")
}

func (b *block) Update(uc *froth.UpdateCtx, msg tea.Msg) {}
func (b *block) Mount(ac *froth.ApplyCtx)                { b.self = ac.Self() }

func TestMountCreatesChildrenBeforeTheFirstRender(t *testing.T) {
	kid := &block{ch: "k", w: 2, h: 1}
	root := &parent{body: "rrrr\nrrrr", kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "kkrr\nrrrr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestMountRunsOncePerSurfaceNotPerFrame(t *testing.T) {
	root := &parent{body: "x", kids: []froth.Surface{&block{ch: "k", w: 1, h: 1}}}
	f := newSized(t, root, 4, 2)

	f.View()
	f.Update(bumpMsg{})
	f.View()

	if root.mounts != 1 {
		t.Errorf("Mount ran %d times, want 1", root.mounts)
	}
	if got := len(root.handles); got != 1 {
		t.Errorf("root has %d children, want 1", got)
	}
}

func TestHandlesAreDistinctAndUsableAsMapKeys(t *testing.T) {
	a, b := &block{ch: "a", w: 1, h: 1}, &block{ch: "b", w: 1, h: 1}
	root := &parent{body: "..", kids: []froth.Surface{a, b}}
	f := newSized(t, root, 4, 2)
	f.View()

	seen := map[froth.Handle]string{f.Root(): "root", a.self: "a", b.self: "b"}
	if len(seen) != 3 {
		t.Errorf("three surfaces produced %d distinct handles: %v", len(seen), seen)
	}
	if a.self == (froth.Handle{}) {
		t.Error("a live surface has the zero handle, which names nothing")
	}
}

func TestLaterSiblingStacksAboveEarlier(t *testing.T) {
	root := &parent{body: "....\n....", kids: []froth.Surface{
		&block{ch: "a", w: 4, h: 2},
		&block{ch: "b", w: 2, h: 1},
	}}
	f := newSized(t, root, 4, 2)

	// b was created second, so it draws over a where they overlap.
	if got, want := f.View().Content, "bbaa\naaaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A subtree stacks as a unit: a deep descendant of an earlier sibling still
// draws below a later sibling, which is the per-parent stacking context ADR
// 0007 claims froth-assigned depth-first ordinals give.
func TestAnEarlierSiblingsSubtreeStaysBelowALaterSibling(t *testing.T) {
	deep := &parent{body: "aaaa\naaaa", kids: []froth.Surface{
		&parent{body: "cccc", kids: []froth.Surface{
			&block{ch: "d", w: 4, h: 1},
		}},
	}}
	root := &parent{body: "....\n....", kids: []froth.Surface{
		deep,
		&block{ch: "z", w: 2, h: 2},
	}}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "zzdd\nzzaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestChildrenRenderAtTheParentsRect(t *testing.T) {
	kid := &block{ch: "k", w: 1, h: 1}
	root := &parent{body: "x", kids: []froth.Surface{kid}}
	f := newSized(t, root, 8, 3)
	f.View()

	if got, want := kid.rect, (froth.Rect{W: 8, H: 3}); got != want {
		t.Errorf("child rect = %+v, want %+v", got, want)
	}
}

// An ApplyCtx stays valid for the whole mount pass, so a surface can be handed
// one by another surface and create a child under a node the pass has already
// walked past. However a surface got into the tree, it mounts before it draws.
type stash struct{ ac **froth.ApplyCtx }

func (s *stash) Render(*froth.RenderCtx, froth.Rect) string { return ".." }
func (s *stash) Update(*froth.UpdateCtx, tea.Msg)           {}
func (s *stash) Mount(ac *froth.ApplyCtx)                   { *s.ac = ac }

type latecomer struct {
	ac  **froth.ApplyCtx
	kid froth.Surface
}

func (l *latecomer) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (l *latecomer) Update(*froth.UpdateCtx, tea.Msg)           {}
func (l *latecomer) Mount(*froth.ApplyCtx)                      { (*l.ac).Create(l.kid) }

func TestASurfaceCreatedLateInTheMountPassIsStillMounted(t *testing.T) {
	var borrowed *froth.ApplyCtx
	late := &block{ch: "L", w: 1, h: 1}
	root := &parent{body: "..", kids: []froth.Surface{
		&stash{ac: &borrowed},
		&latecomer{ac: &borrowed, kid: late},
	}}
	f := newSized(t, root, 2, 1)

	if got, want := f.View().Content, "L."; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if late.self == (froth.Handle{}) {
		t.Error("a surface rendered without ever being mounted")
	}
}
