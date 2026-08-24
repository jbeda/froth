package froth_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// Every handle-scoped context answers the read set, which is what makes
// HandleReader worth being an interface at all.
var (
	_ froth.HandleReader = froth.ReadCtx{}
	_ froth.HandleReader = froth.SelfUpdateCtx{}
	_ froth.HandleReader = froth.SelfApplyCtx{}
	_ froth.HandleReader = froth.ChildCtx{}
	_ froth.HandleReader = froth.AnyCtx{}
)

// prober is a surface that hands its contexts to the test. A context lives
// inside one callback, so being a surface is the only way to reach a mint site.
type prober struct {
	kids    []froth.Surface
	self    froth.Handle
	kidHs   []froth.Handle
	onMount func(ac *froth.ApplyCtx)
	onMsg   func(uc *froth.UpdateCtx)
	onDraw  func(rc *froth.RenderCtx)
}

func (p *prober) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if p.onDraw != nil {
		p.onDraw(rc)
	}
	return "."
}

func (p *prober) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if p.onMsg != nil {
		p.onMsg(uc)
	}
}

func (p *prober) Mount(ac *froth.ApplyCtx) {
	p.self = ac.Self().Handle()
	for _, k := range p.kids {
		p.kidHs = append(p.kidHs, ac.Self().Create(k))
	}
	if p.onMount != nil {
		p.onMount(ac)
	}
}

// A layout has always been handed the container's children as real handles.
// This is the surface that owns them asking the same question.
func TestASurfaceEnumeratesItsOwnChildrenInSiblingOrder(t *testing.T) {
	kids := []froth.Surface{&prober{}, &prober{}}
	root := &prober{kids: kids}
	var got []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) { got = uc.Self().Children() }
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if !slices.Equal(got, root.kidHs) {
		t.Errorf("Children() = %v, want %v", got, root.kidHs)
	}
}

// The registry's own slice never leaves it. A caller that sorted or truncated
// what it was handed would otherwise be rewriting sibling order, which is
// stacking order.
func TestEnumerationHandsBackACopy(t *testing.T) {
	root := &prober{kids: []froth.Surface{&prober{}, &prober{}}}
	var again []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) {
		got := uc.Self().Children()
		got[0] = froth.Handle{}
		again = uc.Self().Children()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if !slices.Equal(again, root.kidHs) {
		t.Errorf("after a caller wrote to what it was handed, Children() = %v, want %v", again, root.kidHs)
	}
}

// Reads are on every tier for any live handle, so a surface can walk in either
// direction from a handle it was given rather than one it created.
func TestAnyTierReadsTheParentAndChildrenOfAStrangersHandle(t *testing.T) {
	grandkid := &prober{}
	kid := &prober{kids: []froth.Surface{grandkid}}
	root := &prober{kids: []froth.Surface{kid}}

	var parent, self froth.Handle
	var children []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) {
		at := uc.At(grandkid.self)
		parent, self = at.Parent(), at.Handle()
		children = uc.At(kid.self).Children()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if parent != kid.self {
		t.Errorf("Parent() = %v, want %v", parent, kid.self)
	}
	if self != grandkid.self {
		t.Errorf("Handle() = %v, want %v", self, grandkid.self)
	}
	if !slices.Equal(children, kid.kidHs) {
		t.Errorf("Children() = %v, want %v", children, kid.kidHs)
	}
}

// The root hangs off nothing, and a leaf has nothing under it. Both are answers
// rather than failures, so neither needs a caller to check anything first.
func TestTheRootHasNoParentAndALeafHasNoChildren(t *testing.T) {
	leaf := &prober{}
	root := &prober{kids: []froth.Surface{leaf}}
	var rootParent froth.Handle
	var leafKids []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) {
		rootParent = uc.Self().Parent()
		leafKids = uc.At(leaf.self).Children()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if rootParent != (froth.Handle{}) {
		t.Errorf("the root's Parent() = %v, want the zero handle", rootParent)
	}
	if len(leafKids) != 0 {
		t.Errorf("a leaf's Children() = %v, want none", leafKids)
	}
}

// The read set does not vary by phase, so a surface deciding what to draw from
// the shape of its subtree asks the same question a mount does.
func TestReadsAnswerInApplyAndInRender(t *testing.T) {
	root := &prober{kids: []froth.Surface{&prober{}, &prober{}}}
	var atMount, atDraw []froth.Handle
	root.onMount = func(ac *froth.ApplyCtx) { atMount = ac.Self().Children() }
	root.onDraw = func(rc *froth.RenderCtx) { atDraw = rc.Self().Children() }
	f := newSized(t, root, 4, 1)

	f.View()

	if !slices.Equal(atMount, root.kidHs) {
		t.Errorf("Children() during Apply = %v, want %v", atMount, root.kidHs)
	}
	if !slices.Equal(atDraw, root.kidHs) {
		t.Errorf("Children() during Render = %v, want %v", atDraw, root.kidHs)
	}
}

// A surface owns itself and its immediate children. Reaching past them is a
// programming error — no data turns a stranger's handle up where a child's was
// meant to go — so it is a panic rather than something to report and survive.
func TestMintingAChildContextForASurfaceThatIsNotOnePanics(t *testing.T) {
	grandkid := &prober{}
	kid := &prober{kids: []froth.Surface{grandkid}}
	root := &prober{kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx) { uc.Child(grandkid.self) }
	f := newSized(t, root, 4, 1)

	var got any
	func() {
		defer func() { got = recover() }()
		f.Update(bumpMsg{})
	}()

	msg := panicText(got)
	if !strings.Contains(msg, "not an immediate child") {
		t.Errorf("panic was %v, want one naming the ownership rule", got)
	}
}

// A handle whose surface has gone is the condition handles exist to survive, so
// the same call that panics for a stranger reports and carries on here.
func TestMintingAChildContextForADeadHandleIsReportedAndDoesNothing(t *testing.T) {
	elsewhere := froth.New(context.Background(), &prober{})
	root := &prober{}
	var children []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) { children = uc.Child(elsewhere.Root()).Children() }
	f, diags := newWatched(t, root, 4, 1)

	f.Update(bumpMsg{})

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagDeadHandle {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagDeadHandle)
	}
	if len(children) != 0 {
		t.Errorf("a context on a dead handle answered %v, want nothing", children)
	}
}

// Every read answers for a handle whose surface has gone, rather than failing:
// that is the whole point of a handle outliving what it names.
func TestReadsOnADeadHandleAnswerEmptyRatherThanFailing(t *testing.T) {
	elsewhere := froth.New(context.Background(), &prober{})
	root := &prober{}
	var parent, self froth.Handle
	var children []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx) {
		at := uc.At(elsewhere.Root())
		parent, self, children = at.Parent(), at.Handle(), at.Children()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if parent != (froth.Handle{}) {
		t.Errorf("Parent() = %v, want the zero handle", parent)
	}
	if self != elsewhere.Root() {
		t.Errorf("Handle() = %v, want the handle it was minted for, %v", self, elsewhere.Root())
	}
	if len(children) != 0 {
		t.Errorf("Children() = %v, want none", children)
	}
}

// A handle-scoped context holds the phase token, so it expires with the
// callback that minted it exactly as the phase context it came from does.
func TestARetainedHandleContextPanicsWhenUsedInALaterPhase(t *testing.T) {
	root := &prober{}
	var kept froth.SelfUpdateCtx
	var got any
	root.onMsg = func(uc *froth.UpdateCtx) { kept = uc.Self() }
	root.onDraw = func(*froth.RenderCtx) {
		if kept == (froth.SelfUpdateCtx{}) {
			return
		}
		func() {
			defer func() { got = recover() }()
			kept.Children()
		}()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})
	f.View()

	msg := panicText(got)
	if !strings.Contains(msg, "must not be retained") {
		t.Errorf("panic was %v, want one naming the retained context", got)
	}
}

// froth hands these out; an application cannot write one down. The zero value
// is what a first unit test reaches for, so it says what is wrong.
func TestAConstructedHandleContextPanicsWithADiagnostic(t *testing.T) {
	var got any
	func() {
		defer func() { got = recover() }()
		froth.AnyCtx{}.Parent()
	}()

	msg := panicText(got)
	if !strings.Contains(msg, "not handed out by froth") {
		t.Errorf("panic was %v, want one naming the constructed context", got)
	}
}
