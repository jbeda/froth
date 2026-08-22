// Package froth is window and surface management for Bubble Tea. An application
// hands froth a root surface and forwards Init, Update and View to it; froth
// renders the surface tree through Lip Gloss's compositor and routes messages to
// the surfaces that should see them.
package froth

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Froth owns a surface tree and drives it. An application holds one by pointer
// and forwards Bubble Tea's three model methods to it, so copying the
// application model by value — which Bubble Tea does on every update — copies a
// pointer rather than the tree.
type Froth struct {
	ctx  context.Context
	root Surface
	size Size

	phase phase
	epoch uint64
}

// New builds a froth instance around a root surface. ctx is the root of the
// per-surface context tree: work a surface starts is cancelled when the surface
// is destroyed, and all of it when ctx is.
func New(ctx context.Context, root Surface) *Froth {
	if ctx == nil {
		ctx = context.Background()
	}
	if root == nil {
		panic("froth: New called with a nil root surface")
	}
	return &Froth{ctx: ctx, root: root}
}

// Init returns the work froth needs done before the first frame. Forward it
// from the application model's Init.
func (f *Froth) Init() tea.Cmd { return nil }

// Update delivers one message and returns the work the surfaces that saw it
// asked for. Forward it from the application model's Update.
//
// The terminal size is froth's own business: a resize updates the rect surfaces
// render into and is never delivered to a surface as a message.
func (f *Froth) Update(msg tea.Msg) tea.Cmd {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		f.size = Size{W: ws.Width, H: ws.Height}
		return nil
	}
	return f.deliver(f.root, msg)
}

// View composites the surface tree into a frame. Forward it from the
// application model's View.
func (f *Froth) View() tea.View {
	return tea.NewView(f.render())
}

// deliver runs the Deliver phase for one message and one surface.
func (f *Froth) deliver(s Surface, msg tea.Msg) tea.Cmd {
	uc := &UpdateCtx{phaseToken: f.enter(phaseDeliver), ctx: f.ctx}
	defer f.leave()

	s.Update(uc, msg)
	return tea.Batch(uc.cmds...)
}

// render draws every surface and composites the result. Even with one surface
// the frame goes through Lip Gloss's canvas and compositor rather than being
// assembled as a string, so clipping, wide graphemes and Z ordering are the
// compositor's job from the first frame onwards.
func (f *Froth) render() string {
	screen := Rect{W: f.size.W, H: f.size.H}
	if screen.Empty() {
		return ""
	}

	layers := f.layers(screen)
	canvas := lipgloss.NewCanvas(screen.W, screen.H)
	canvas.Compose(lipgloss.NewCompositor(layers...))
	return canvas.Render()
}

// layers renders each surface and places it as a layer at its own rect. Z is a
// unique global ordinal assigned by froth rather than by the surfaces, because
// Lip Gloss sorts layers by Z across the whole tree and its sort is not stable.
func (f *Froth) layers(screen Rect) []*lipgloss.Layer {
	token := f.enter(phaseRender)
	defer f.leave()

	rc := &RenderCtx{phaseToken: token, rect: screen}
	content := f.root.Render(rc, screen)

	root := lipgloss.NewLayer(content).
		ID("root").
		X(screen.X).
		Y(screen.Y).
		Z(0)
	return []*lipgloss.Layer{root}
}
