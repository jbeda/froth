# Kitchen sink: an API-first sketch

> **Status: design-time artifact, written before any code existed.** Its job was to find
> ergonomic problems while changing them was still free, and it did — see the findings.
> Where this and the code disagree, **the code wins**; fix this document in the same
> change. Delete it once the example application and godoc cover the same ground.
>
> Most of what is below is unbuilt. M1 shipped `Create` (without options yet), `Mount`,
> `SetLayout`, `Focus`, `Post`, `Run`, `Ctx` and three of the four context types
> (`MeasureCtx` waits for M7); everything concerning intents, modals, the Environment,
> producers, reconcile, destroy, bindings and the bubbles adapter is still design.
> [`milestone-log.md`](./milestone-log.md) records what shipped and where it diverged.

Client code for a sample app, written as if froth existed. **None of this compiles.**
Its job is to make the API's ergonomics visible while changing them is still free.

The app is a generic dev-tool TUI: a tiled sidebar and main pane, a virtualized log
that a goroutine feeds, a command palette that dismisses on click-outside, and a
confirm dialog that returns a result to whoever opened it — deliberately while an
async request is still in flight.

Widgets come from [bubbles](https://github.com/charmbracelet/bubbles) v2 through an
adapter rather than being written from scratch. Findings are collected at the end;
several contradict decisions we made.

## Bootstrap

```go
type model struct{ f *froth.Froth }

func newModel(deps Deps) model {
    return model{f: froth.New(deps.Ctx, &appRoot{deps: deps},
        froth.OnDiagnostic(func(d froth.Diagnostic) { log.Print(d) }),
    )}
}

func (m model) Init() tea.Cmd  { return m.f.Init() }
func (m model) View() tea.View { return m.f.View() }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    return m, m.f.Update(msg)   // froth is a pointer; the value copy is free
}
```

The root is an ordinary surface, passed to `froth.New`. There is no separate bootstrap
concept: the app assembles itself in the root's `Mount`, using the same Apply machinery
as every later structural change.

```go
func (a *appRoot) Mount(ac *froth.ApplyCtx) {
    ac.SetEnv(ThemeKey, a.deps.Theme)

    side := ac.Create(newSessionList(), froth.Weight(1))
    ac.Create(&mainPane{}, froth.Weight(3))
    ac.SetLayout(layout.Row(layout.Gap(1)))

    ac.Focus(side)
}

func (m *mainPane) Mount(ac *froth.ApplyCtx) {
    ac.Create(froth.NewScroller(), froth.Clip())   // which mounts the transcript
}
```

`Create` makes a child of *self* — there is no parent parameter, so a surface cannot
reach past its own children, and each surface defines its own contents. The context
knows which surface it belongs to, so `SetEnv`, `SetLayout` and `Focus` need no handle
for the common case either.

The whole Bubble Tea integration is three forwarding methods.

The context passed to `froth.New` is the root of a per-surface context tree: froth
derives a child context for every surface and cancels it when that surface is
destroyed. A surface issuing an RPC uses its own context and gets cancellation on
teardown for free.

## What a surface is

Required — two methods, and every concrete type in this sketch satisfies them:

```go
type Surface interface {
    Render(rc *RenderCtx, r Rect) string
    Update(uc *UpdateCtx, msg tea.Msg)
}
```

Optional, discovered by type assertion, each earning its place from a decision:

```go
type Mounter interface   { Mount(ac *ApplyCtx) }                      // self-assemble children
type Reconciler interface{ Reconcile(ac *ApplyCtx) }                  // restructure on demand
type Unmounter interface { Unmount(ac *ApplyCtx) }                    // release non-froth resources
type Measurer interface  { Measure(mc *MeasureCtx, offer Size) Size } // multi-pass layout
type Binder interface    { Bindings() []Binding }                     // shortcuts + help
type IntentSink interface{ Apply(ac *ApplyCtx, in Intent) bool }      // handle descendants' intents
type ItemSource interface {                                           // virtualized content
    ItemCount() int
    RenderItem(rc *RenderCtx, i, width int) string
}
```

`ItemSource` is an optimization a `Scroller` parent exploits, not a replacement for
`Render`: a virtualized surface parented anywhere else still renders normally.

`Layout` is deliberately absent — it is an object attached to a container, not
something a surface implements.

## Adapting bubbles

bubbles components are value-in/value-out (`Update(tea.Msg) (Model, tea.Cmd)`), which
composes cleanly: the adapter holds the component as a plain field on a surface froth
holds by pointer.

The adapter lives in its own package, `froth/bubbleadapt`, so froth's core never
takes a dependency on bubbles — apps that don't want it don't pull it in.

`Surface[M]` is the adapter: it holds the component by value, carries the
per-component glue as plain function fields, and implements froth's surface methods.
Go can express "a type whose `Update` returns its own type" directly, so no reflection
or codegen is involved:

```go
type Component[M any] interface {
    Update(tea.Msg) (M, tea.Cmd)
    View() string
}

type Surface[M Component[M]] struct {
    M M              // the wrapped component, exported so surfaces can query it

    Size   func(*M, int, int)
    Focus  func(*M, bool) tea.Cmd
    Cursor func(M) *tea.Cursor
}

func (s *Surface[M]) Render(rc *froth.RenderCtx, r froth.Rect) string {
    if s.Size != nil {
        s.Size(&s.M, r.W, r.H)          // froth is immediate-mode: size every frame
    }
    if s.Cursor != nil {
        if c := s.Cursor(s.M); c != nil {
            rc.Cursor(froth.Pt(c.X, c.Y))
        }
    }
    return s.M.View()
}

func (s *Surface[M]) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    var cmd tea.Cmd
    s.M, cmd = s.M.Update(msg)
    uc.Run(cmd)          // nothing to report: shortcuts resolved before delivery
}
```

`list.Model` satisfies `Component[list.Model]` as it stands — `Update(tea.Msg)
(Model, tea.Cmd)` and `View() string` — so nothing in bubbles has to change.

There is no constructor and no options: the glue is exported fields you fill in.

What is *not* uniform across bubbles is sizing (`SetWidth` only, versus `SetSize`),
focus (some components have `Focus`/`Blur`, some have no focus concept), and cursor
reporting. So the adapter takes that glue explicitly:

```go
in := bubbleadapt.Surface[textinput.Model]{
    M:      textinput.New(),
    Size:   func(m *textinput.Model, w, _ int) { m.SetWidth(w) },
    Focus:  func(m *textinput.Model, on bool) tea.Cmd {
        if on { return m.Focus() }
        m.Blur()
        return nil
    },
    Cursor: func(m textinput.Model) *tea.Cursor { return m.Cursor() },
}
```

Type parameters can't be inferred on a struct literal, so `[textinput.Model]` is
written out. froth ships the common components pre-filled, and those are the only
constructors — they earn the name by saying which component you get:

```go
in   := bubbleadapt.TextInput(textinput.New())
rows := bubbleadapt.List(list.New(items, delegate, 0, 0))
```

The adapter owns three responsibilities the component can't: calling the sizing glue
from `Render` before `View` (froth is immediate-mode, so size arrives every frame),
forwarding froth's focus changes into `Focus`/`Blur` so the two focus models can't
desync, and translating the component's reported cursor into a froth cursor claim.

## A surface

```go
type sessionList struct {
    rows bubbleadapt.Surface[list.Model]
}

func newSessionList() *sessionList {
    return &sessionList{rows: bubbleadapt.List(list.New(nil, sessionDelegate{}, 0, 0))}
}

func (s *sessionList) Render(rc *froth.RenderCtx, r froth.Rect) string {
    return s.rows.Render(rc, r)          // sizes, views, claims cursor
}

func (s *sessionList) Bindings() []froth.Binding {
    return []froth.Binding{{
        Action: ActionDeleteSession,
        Keys:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete session")),
    }}
}

func (s *sessionList) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    switch m := msg.(type) {
    case froth.ActionMsg:                 // resolved before any key reached us
        if m.Action == ActionDeleteSession {
            uc.Raise(ConfirmDeleteIntent{Row: s.rows.M.SelectedItem().(row)})
            return
        }
    case sessionsLoadedMsg:               // arrives via the envelope, addressed here
        uc.Run(s.rows.M.SetItems(m.Items))
        return
    }
    s.rows.Update(uc, msg)                // j/k, filtering, paging: all bubbles'
}
```

Composition, not inheritance. The surface adds only what bubbles doesn't do — one
declared action, and receiving its own async load — and delegates everything else.
`s.rows.M` reaches the wrapped component for queries like `SelectedItem`.

Note it never matches a key. It declares a binding and handles the `ActionMsg` froth
resolved, so remapping `d` to something else touches no surface code.

## Opening a modal, and getting the answer back

The list opens it directly. It needs nothing back — the answer arrives later as a
message — so no handle changes hands and the no-eager-minting rule is untouched:

```go
case froth.ActionMsg:
    if m.Action == ActionDeleteSession {
        uc.OpenModal(newConfirmDialog(s.rows.M.SelectedItem().(row).Label))
        return
    }
```

froth parents it to root, marks it modal and a focus scope, focuses it, and wires its
reply address back to the requesting surface. Same shape as `uc.Close()`: recorded
during Deliver, committed during Apply. Root never learns a confirm dialog exists.

```go
func (d *confirmDialog) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    switch m := msg.(type) {
    case OKPressed:
        uc.Reply(ConfirmResult{OK: true})
        uc.Close()
    case CancelPressed:
        uc.Reply(ConfirmResult{OK: false})
        uc.Close()
    case froth.ActionMsg:
        if m.Action == froth.ActionCancel {      // esc, per froth's stock bindings
            uc.Reply(ConfirmResult{OK: false})
            uc.Close()
        }
    }
}
```

And back in the list, as an ordinary message — same frame, because the update loop
drains before rendering:

```go
case ConfirmResult:
    if m.OK {
        uc.Run(deleteSessionCmd(s.rows.M.SelectedItem().(row).ID))
    }
```

The async delete is still in flight when the dialog is destroyed. Its result is
addressed to the *list*, which is alive, so it lands normally. Had the list been
destroyed too, it would be a counted drop with nothing to clean up.

### When an ancestor owns the policy

Opening directly is right when a dialog is self-contained. Route through an intent
instead when something above genuinely has to decide — only one modal at a time, a
parent other than root, or dependencies the raiser can't supply:

```go
uc.Raise(ConfirmDeleteIntent{Row: s.rows.M.SelectedItem().(row)})
```

```go
func (a *appRoot) Apply(ac *froth.ApplyCtx, in froth.Intent) bool {
    switch in := in.(type) {
    case ConfirmDeleteIntent:
        dlg := ac.Create(ac.Root(), newConfirmDialog(in.Row.Label),
            froth.Modal(), froth.FocusScope(), froth.RepliesTo(ac.Sender()))
        ac.Focus(dlg)
        return true
    }
    return false
}
```

`ac.Sender()` is the handle that raised the intent, so the reply is wired without the
list ever naming itself.

The cost of allowing both: once any surface can open a root-parented modal directly,
there is no single chokepoint where an app could enforce modal policy. If that turns
out to matter it wants an explicit hook in froth, not a convention that everyone
routes through root.

### Later: a dialog helper library

The ergonomic end point is the dialog owning its own opening, so a caller writes

```go
confirm.Ask(uc, "Delete this session?")     // later receives a confirm.Result
```

and never constructs the dialog, names root, or knows it is modal. That is sugar over
`uc.OpenModal` and wants a small library of stock dialogs — confirm, prompt, pick —
built once the core is stable. Not v1, but the shape it needs is already here.

## Where an intent goes

froth walks parent-ward from the raising surface, calling `Apply` on each ancestor
that implements `IntentSink`. The first to return `true` claims it — nearest ancestor
wins, the same precedence as shortcut resolution. `ConfirmDeleteIntent` reaches the
app root only because nothing between the list and root cares about it.

The bool survives here where it died on `Update`, and the asymmetry is the point:
intent handling is chain-of-responsibility, so "will anyone deal with this" is
genuinely unknown until someone says yes. Raw input has one correct destination
decided in advance, so it has nothing to report.

Four rules the walk needs:

- **An unhandled intent is reported, not dropped.** Raising something no ancestor
  claims is almost always a missing case or a surface parented somewhere unexpected.
  It surfaces through the same diagnostic hook as a panicking surface.
- **`Sender()` names the raiser**, not whichever ancestor the walk is currently
  visiting — otherwise `RepliesTo(ac.Sender())` wires the reply to the wrong surface.
- **Intents apply before queued destroys.** A handler may call both `uc.Raise` and
  `uc.Close`; if the destroy committed first, `Sender()` would name a corpse and the
  modal would reply into a dead handle.
- **An ancestor may raise while handling, and that queues rather than recurses.**
  Appended to the intent queue and processed in the same drain, under the same
  iteration cap as message delivery — otherwise Apply nests, which is the reentrancy
  excluded everywhere else.

## The Environment

Ambient values a surface reads from its ancestors. Scoped by subtree, not global:
resolution walks root-ward from the reader and takes the nearest binding, so a subtree
can override what is above it.

Keys carry their own type, so both ends are checked:

```go
var ThemeKey = froth.NewKey[theme.Theme]("theme")

ac.SetEnv(ThemeKey, deps.Theme)   // only accepts a theme.Theme; sets on self
th := rc.Env(ThemeKey)            // returns a theme.Theme, no assertion
```

These are generic methods on the concrete context types, which Go 1.27 allows. No
handle is passed: the context knows which surface it belongs to.

A key may carry a default, which removes the missing-value failure mode entirely:

```go
var DensityKey = froth.NewKeyDefault("density", layout.Comfortable)
```

What remains dynamic is *presence* for keys without defaults. Go can't check that
without threading values through constructors, which is exactly the tradeoff the
Environment exists to avoid — so it stays a small, deliberate hole rather than one
that got designed in by accident.

### Reads are broad, writes are narrow

```go
func (rc *RenderCtx) Env[T any](k *Key[T]) T          // and on every other context
func (ac *ApplyCtx) SetEnv[T any](k *Key[T], v T)     // sets on self
func (ac *ApplyCtx) SetEnvOn[T any](h Handle, k *Key[T], v T)   // an immediate child
func froth.Env[T any](r EnvReader, k *Key[T]) T       // for cross-context helpers
```

The common form takes no handle at all, so setting on an ancestor is not expressible
rather than merely forbidden. `SetEnvOn` exists for the deliberate case of giving one
child something different, and it accepts only an immediate child. Setting on yourself
already covers your whole subtree, since resolution walks root-ward.

The package-level `froth.Env` remains for a helper called from more than one phase.
**Interface methods still cannot have type parameters**, so `EnvReader` carries an
untyped lookup and the free function layers typing over it — same guarantees, one
awkward shape retained for one real case.
That is also the honest form of "a surface can own an ambient value": it can, for the
scope that is its own. The theme picker still cannot set root's theme, because root
isn't its.

`EnvReader` is a small interface all four context types satisfy, so reading works in
Render, Update, Apply and Measure alike. Writing takes the concrete `*ApplyCtx`, so
calling `SetEnv` from a render is a compile error rather than a runtime one.

`SetEnv` is deliberately *not* available on `UpdateCtx`, even though `Destroy` is. If
it queued during Deliver, a read later in the same handler would return the old value —
the live-tree-versus-rendered-frame split, applied to something that reads like a plain
variable lookup. Apply-only buys a guarantee instead: **within a frame, the Environment
binding does not change.** Every surface delivering, measuring and rendering in that
frame resolves the same values, with no ordering rules to document.

The cost is small, because Environment values change rarely by construction. A surface
that wants to change one for its subtree raises an intent and the ancestor that owns
the value sets it during Apply — which is the right place anyway, since the value was
never the raiser's to own.

**This is best effort, and Go is why.** The guarantee covers the *binding*, not the
value behind it: nothing stops a surface storing a pointer into the Environment and
mutating what it points at, and Go has no `const` to prevent it. So put values in the
Environment rather than handles to mutable state, and prefer types that are immutable
in practice. Sharing something mutable through it opts out of the guarantee, silently.

### When to use it

All three must hold:

1. **Many surfaces across a subtree read it.** For a single reader this is action at a
   distance; pass a constructor parameter.
2. **The reader only reads; an ancestor owns it.** If the surface mutates it, it is
   state, and shared mutable state through an ambient channel is the worst option.
3. **It can change during the reader's lifetime and a stale copy would be wrong.**
   This is what justifies the Environment over constructor injection at all. If the
   value never changes, a constructor parameter is *better* — explicit, greppable, and
   checked at compile time.

Canonically in: theme, terminal capabilities, locale, a clock for testability, a
logger. Canonically not: the current session, the selected item, the open document.

The smell test: **if two surfaces could reasonably disagree about the right value, it
isn't ambient.** Theme has one right answer per scope; "the current document" doesn't,
which is why it belongs to a surface.

The anti-pattern is the Environment as an app-state bus. It is reachable from
everywhere, so it accumulates, and what you lose is every dependency being invisible —
no signal that a surface needs something, and headless tests populating an environment
they can't discover from the code. Domain state lives on surfaces, moves up through
intents, and comes back as messages.

### Worked example: a theme picker

The picker cannot call `SetEnv` — it does not own `ThemeKey`, and it is in Deliver. It
raises, and the owner commits during Apply. No state is held in limbo, because the
intent *is* the pending value.

```go
func (p *themePicker) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    if m, ok := msg.(froth.ActionMsg); ok {
        switch m.Action {
        case ActionAccept:
            uc.Raise(SetThemeIntent{Theme: p.selected(), Commit: true})
            uc.Close()
            return
        case ActionCancel:
            uc.Raise(RevertThemeIntent{})
            uc.Close()
            return
        }
    }
    p.list.Update(uc, msg)
    if sel := p.selected(); sel != p.previewing {   // arrowed: preview live
        p.previewing = sel
        uc.Raise(SetThemeIntent{Theme: sel})
    }
}
```

```go
func (a *appRoot) Apply(ac *froth.ApplyCtx, in froth.Intent) bool {
    switch in := in.(type) {
    case SetThemeIntent:
        if a.saved == nil {                          // first preview: stash the original
            t := ac.Env(ThemeKey)
            a.saved = &t
        }
        ac.SetEnvOn(ac.Root(), ThemeKey, in.Theme)
        if in.Commit {
            a.saved = nil
        }
        return true
    case RevertThemeIntent:
        if a.saved != nil {
            ac.SetEnvOn(ac.Root(), ThemeKey, *a.saved)
            a.saved = nil
        }
        return true
    }
    return false
}
```

Raise in Deliver, commit in Apply, redraw in Render — one frame, no skew, every surface
picking up the new theme at once because the binding changed before anything rendered.

Two things this shows beyond the mechanics. **The owner holds the original, not the
picker**, so a picker destroyed mid-preview leaves nothing dangling — cancel is the
owner's job because the value was always the owner's. And **live preview raises an
intent per keypress**, which is fine mechanically but means "Environment values change
rarely" describes how often it is *justified*, not a performance assumption to lean on.

### Modals see a shallower environment

A modal parents to the root, so it resolves against root's bindings — not those of the
surface that opened it. A dialog opened from a subtree with a theme override renders
with the *root* theme.

That is the correct default rather than a wart. Making the environment follow the
opener instead of the parent would mean environment resolution using a different chain
than everything else, which is the parent-versus-owner split
[ADR 0006](../adr/0006-one-parent-relation.md) deliberately declined. One relation,
one resolution rule.

When an opener does need to propagate something, it sets it on the modal explicitly at
open — narrow, visible, and local to the case that needs it.

## Mount, Reconcile and Unmount

`Mount` runs **once**, during the Apply in which the surface is created — after its
handle exists, before its first render. Never per-frame; `RenderCtx` has no `Create`
to begin with. Nothing re-triggers it: entering the tree happens once, and since
`Reparent` was cut a surface never moves within the tree either — its parent is fixed
for its lifetime.

It is not the only way to get children, and it is not for dynamic ones. Three routes,
with different jobs:

- **The creator does it inline** — `h := ac.Create(root, &dlg{}); ac.Create(h, btn)`.
  Right when the *creator* decides the structure.
- **`Mount`** — right when the *surface* defines its own structure. A confirm dialog
  always has OK and Cancel, and its creator shouldn't have to know that.
- **Later, during any Apply** — a container adding a row when you click "+" creates
  that child in response to an action, nowhere near Mount.

Mount recurses: a surface created inside `Mount` gets its own `Mount` immediately, so
a subtree assembles before the first render. Bounded by depth, but a surface that
mounts a child of its own type would recurse forever, so it sits under the same
iteration cap as the Deliver/Apply drain.

### Reconcile: as-needed Apply

`Mount` is one-time Apply init. `Reconcile` is the as-needed version: the surface calls
`uc.NeedsReconcile()` during Deliver, and froth calls it during Apply with a real
`ApplyCtx` — real handles, immediate effect, everything Apply can do.

It exists because a surface doing serious restructuring in response to input has
nowhere else to go. Creating a child is Apply-only, so without this the only route is
an ancestor handling an intent, which forces ancestors to know about structure that
isn't theirs.

What matters is how it changes the way a restructure is written. You don't queue
operations, you express desired state and make the tree match it:

```go
func (t *tabStrip) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    // ... input handling sets ordinary state ...
    t.want = computeTabs(t.model)
    uc.NeedsReconcile()
}

func (t *tabStrip) Reconcile(ac *froth.ApplyCtx) {
    // create what's missing, destroy what's gone, reorder — with real handles
}
```

That is the answer to "how does it remember what it wanted to do": it doesn't, because
the desired state *is* the memory and reconcile is a function of it.

**Rejected alternatives.** `uc.Create(surface, opts...)`, queued and returning no
handle: fine for one child, but a real restructure creates several, destroys others and
reorders, and with no handles coming back the surface ends up keeping a journal of what
it asked for — worse than the problem. `uc.Defer(func(ac *ApplyCtx))`: a closure
captures what was known at Deliver time, where a method reads state fresh at Apply
time; it is also neither greppable nor testable. Delivering intents to the raiser
itself: free, but it turns `Raise` into a general "run later" primitive and fills the
intent vocabulary with private scheduling types no ancestor will ever match, so a
reader can no longer tell a request upward from a note to self.

Mount and Reconcile stay separate rather than unifying into one idempotent builder,
because Mount also does one-time non-structural setup like starting producers, and
running that again would restart them.

### Close and Destroy

`uc.Close()` is self-destroy sugar: it records a destroy of the calling surface, which
commits at Apply, exactly like `ac.Destroy(self)` would. `ac.Destroy(h)` destroys any
handle the caller **owns** — itself or one of its immediate children (see Ownership in
the Reference) — and takes effect immediately when called during Apply, or at the next
Apply when recorded during Deliver. Destruction cascades to descendants either way, so
removing a deep subtree means destroying your own child and letting the cascade do the
rest.

### Unmount

`Unmount` is the mirror of Mount, called during Apply when a surface is destroyed, **leaf-up**
so a child can still reach its parent while tearing down — the opposite of Mount's
order.

It should be rare. froth already cancels producers, cascades destruction to children,
and cancels the surface's context, so in-flight RPCs abort without anyone writing
teardown. `Unmount` is for resources froth doesn't know about — a file handle, a
registration in some external registry — and most surfaces will never implement it.

## Controls are surfaces

```go
func (d *confirmDialog) Mount(ac *froth.ApplyCtx) {
    ac.Create(froth.NewButton("OK", OKPressed{}))
    ac.Create(froth.NewButton("Cancel", CancelPressed{}))
    ac.SetLayout(layout.Row(layout.End, layout.Gap(2)))
}

func (d *confirmDialog) Render(rc *froth.RenderCtx, r froth.Rect) string {
    return rc.Env(ThemeKey).Prompt(d.label)
    // buttons are children; froth composites them over this body
}
```

Tab moves between the buttons because they are surfaces in a focus scope. Nothing here
mentions hit-testing, click regions, or focus rings.

**Tab order is child order.** There are two orders in froth and no third: the child list
drives what a layout arranges, the depth-first walk, and keyboard traversal; `Z` affects
stacking alone. `NoTabStop` takes a surface out of traversal without moving it. Win32 has
no separate focus index either — its dialog manager searches in the order controls were
created — and HTML's `tabindex` is the standing example of what a third order costs. Button is froth's own — bubbles
has no button, because in a single-model world a button is just a styled string.

`Mount` is new: a surface needs somewhere to create children, and it can't be the
constructor, which has no `ApplyCtx` and no handle yet.

## A goroutine-driven pane

A producer is a **step function** froth calls in a loop it owns, not a loop the surface
writes:

```go
func (l *logPane) Mount(ac *froth.ApplyCtx) {
    ac.Produce(l.nextLine, froth.BatchedFunc(mergeLines))
}

// nextLine blocks until the next line. froth owns the loop, the goroutine, and
// cancellation — and a test can call this directly with no concurrency at all.
func (l *logPane) nextLine(ctx context.Context) (logLineMsg, error) {
    select {
    case <-ctx.Done():
        return logLineMsg{}, ctx.Err()
    case line, ok := <-l.tailer.Lines():
        if !ok {
            return logLineMsg{}, froth.EndOfStream
        }
        return logLineMsg{Line: line}, nil
    }
}

// mergeLines collapses everything pending for this surface into one message.
func mergeLines(pending []logLineMsg) logLinesMsg {
    out := make([]string, len(pending))
    for i, m := range pending {
        out[i] = m.Line
    }
    return logLinesMsg{Lines: out}
}
```

froth starts the loop at mount, cancels it at destroy, tags each result with the handle,
and drops results for a surface that has died. Returning `EndOfStream` stops the loop
cleanly; any other error stops it and reports through the diagnostic hook.

Inverting the loop this way is what makes producers testable. With a
surface-written loop, a test can only interrupt it from outside; with a step function,
the surface's contribution is an ordinary function a test calls with a pre-filled
channel, and the only loop in the system is froth's.

The surface still never touches its own state from the goroutine — the step function
returns a message, and that message arrives in `Update` on the loop like anything else.

### Delivery discipline

Three options, and the default is the boring one:

```go
ac.Produce(next)                       // every message delivered, in order
ac.Produce(next, froth.Batched[T]())   // pending collapse to froth.Batch[T]
ac.Produce(next, froth.BatchedFunc(f)) // pending collapse via your own merge
ac.Produce(next, froth.Coalesced[T]()) // only the newest pending survives
```

**Coalescing drops content and batching does not**, which is the distinction worth
getting right. A progress percentage wants coalescing — only the latest matters. Log
lines want batching — every one matters, but not one wakeup each. Defaulting to either
silently corrupts the other case, so the default is neither.

`Produce` is a free function rather than a method for the same reason `SetEnv` is:
**Go methods cannot have type parameters**, and without one the emit channel would be
`tea.Msg` and every option would be untyped. Second time this constraint has shaped the
API, which is worth noticing as a pattern rather than an accident.

**Watch the type change.** With `BatchedFunc(mergeLines)` the producer emits
`logLineMsg` but the surface receives `logLinesMsg` — write a case for the singular and
you will silently never match. `Batched[T]()` has the same property in a more visible
form, delivering `froth.Batch[logLineMsg]`. Worth a diagnostic: a surface with a
producer whose delivered type it never handles is almost certainly a bug.

## Virtualized content, anchored

```go
func (t *transcript) ItemCount() int { return len(t.entries) }

func (t *transcript) RenderItem(rc *froth.RenderCtx, i, width int) string {
    return t.entries[i].render(width)      // froth caches by (i, width)
}
```

Two methods, no scroll state, no total-height arithmetic. The scroller parent owns the
anchor and asks only for the items the viewport needs.

This one stays froth-native rather than adapted: bubbles' `viewport` takes its content
as `SetContent(string)`, which means materialising the whole transcript every frame —
exactly the cost virtualization exists to avoid.

## Click-outside dismissal

Click-outside only works because the palette **holds capture**. Without it a click that
lands elsewhere is hit-tested to whatever is under the cursor and the palette never sees
it. Capture is claimed when the palette is created:

```go
func (p *palette) Mount(ac *froth.ApplyCtx) {
    ac.Capture()                         // all mouse input, wherever it lands
}

func (p *palette) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    switch m := msg.(type) {
    case froth.CaptureLostMsg:
        uc.Close()                       // abandon, never complete
        return
    case tea.MouseClickMsg:
        if !uc.Rect().Contains(m.X, m.Y) {
            uc.Close()
            return
        }
    }
    p.filter.Update(uc, msg)             // created with froth.TextSink()
}
```

The routing rule this depends on: **while any surface holds capture, every mouse message
goes to it regardless of position; otherwise mouse messages are hit-tested.** That is
still one destination per message — capture changes which one, it doesn't broadcast.

`uc.Rect()` reads this surface's rect from the *rendered frame* — the last frame that
actually drew, which is what the click was resolved against.

Capture releases on destroy, when a modal opens, or explicitly, and each of those
delivers `CaptureLostMsg` first.

## A nested text input claiming the cursor

```go
type editor struct{ in bubbleadapt.Surface[textinput.Model] }

func (e *editor) Render(rc *froth.RenderCtx, r froth.Rect) string {
    return e.in.Render(rc, r)   // adapter claims the cursor from in.M.Cursor()
}

func (e *editor) Update(uc *froth.UpdateCtx, msg tea.Msg) {
    e.in.Update(uc, msg)
}
```

The component reports a cursor in its own coordinates; the adapter turns that into a
froth claim; froth translates it through every ancestor offset, ignores it if this
surface isn't focused, and suppresses it if it lands outside the visible region. There
is no cursor state to keep in sync, because the claim exists only for the frame that
made it.

## Reference

Everything the sketch uses, in one place. This is the API surface these examples imply,
not a finished specification.

### Contexts

Each phase gets its own type, so an illegal call is a compile error rather than a
runtime one. Every context can read the Environment and report the caller's own handle.

| Context | Phase | Also offers |
|---|---|---|
| `*RenderCtx` | Render | `Cursor(Point)` — claim the terminal cursor · `Rect()` · `Focused()` — whether this surface holds focus |
| `*MeasureCtx` | Measure | nothing beyond the common set; deliberately the narrowest |
| `*UpdateCtx` | Deliver | `Ctx()` this surface's context · `Run(tea.Cmd)` · `Post(Handle, tea.Msg)` · `Focus(Handle)` / `FocusSelf()` · `Raise(Intent)` · `Reply(tea.Msg)` · `Close()` · `OpenModal(Surface, ...Option)` · `Capture()` / `ReleaseCapture()` · `NeedsReconcile()` · `Rect()` |
| `*ApplyCtx` | Apply | `Create(Surface, ...Option) Handle` — a child of self · `Destroy(Handle)` · `SetLayout(Layout)` · `Focus(Handle)` / `FocusSelf()` · `Post(Handle, tea.Msg)` · `Capture()` · `Produce(next, ...ProducerOpt)` · `Root()` · `Sender()` — the handle that raised the intent being applied |

Every context knows the surface it belongs to and exposes it as `Self()`, so the
self-relative operations take no handle. Structural calls made on an `*UpdateCtx` are
recorded and commit at Apply; the same calls on an `*ApplyCtx` take effect immediately.

### Core types

```go
type Handle struct{ ... }        // opaque, comparable, zero value is "none"
type Rect   struct{ X, Y, W, H int }
type Point  struct{ X, Y int }
func Pt(x, y int) Point

type Intent any                  // no method requirement; the type is the contract
type Surface interface{ ... }    // see "What a surface is"
type Layout  interface{ Arrange(space Rect, children []Child) []Rect }  // screen rects in, screen rects out

type Key[T any] struct{ ... }    // Environment key carrying its own value type
func NewKey[T any](name string) *Key[T]
func NewKeyDefault[T any](name string, def T) *Key[T]

type Action  string              // stable identity for a shortcut
type Binding struct {
    Action Action
    Keys   key.Binding           // default keys and help text
    SurvivesModal bool           // still resolves from outside an open modal
}
type ActionMsg struct{ Action Action }
```

### Create options

Passed to `Create` and `OpenModal`, stored as handle metadata:

```go
froth.Weight(n)        // relative share of the parent layout's space
froth.Float()          // placed against the screen, not by the parent's layout
froth.Z(n)             // stacking among siblings; default 0, ties by creation order
froth.Clip()           // clip children to this surface's rect
froth.Modal()          // take input for the parent's whole subtree
froth.FocusScope()     // keyboard navigation cycles within this subtree
froth.TextSink()       // bare printable keys are not matched as shortcuts here
froth.RepliesTo(h)     // where uc.Reply() sends
froth.AutoRaise()      // raise among siblings on focus; off by default
froth.NoTabStop()      // skip in keyboard navigation
```

### Ownership

One rule governs `Create`, `Destroy` and `SetEnv`:

> **A surface owns itself and its immediate children.**

Mostly this is structural rather than checked: `Create` makes a child of self and takes
no parent, `SetEnv` sets on self and takes no handle. Only the operations that must name
a target — `Destroy`, `SetEnvOn` — validate that the target is an immediate child.

A surface can also only destroy a handle it *holds*, and `OpenModal` hands none back, so
a surface reconciling its own children cannot reach a modal opened beneath it even by
accident. That is why nothing needs to exempt modals from reconcile: froth destroys
nothing on its own, and there is no way to enumerate children to sweep them.

Ownership is parenthood. It is deliberately *not* "whatever you created" — that would be
a second relation running alongside the parent link, which is the parent-versus-owner
split [ADR 0006](../adr/0006-one-parent-relation.md) declined. And it is not "your whole
subtree", which would let a grandparent delete a grandchild out from under its parent,
breaking the invariant `Reconcile` depends on: a surface computing desired state for its
children must be the only thing editing them.

Nothing is lost. Destruction cascades, so a deep subtree goes when you destroy your own
child. The one operation you cannot perform is surgically removing a grandchild while
keeping its parent — and that belongs to the parent, which can expose it as an intent.

Two operations cross the boundary because froth performs them rather than a surface:
`OpenModal` parents to the root, and destruction cascades past it.

**Known future tension.** If froth grows implicit grouping surfaces — containers that
exist only to carry a layout, the way a `div` does — then a surface's immediate children
stop being what it created and this rule needs revisiting. Not a problem today, because
every surface in the tree is one somebody asked for.

### Diagnostics

```go
froth.New(ctx, root, froth.OnDiagnostic(func(d froth.Diagnostic) { ... }))
```

One hook, receiving everything froth can detect but not decide about: a surface that
panicked and was quarantined, an intent no ancestor claimed, a message delivered to a
dead handle, a producer whose delivered type its surface never handles, and the
iteration cap tripping on a message or reconcile cycle. Each is nearly always a bug and
none of them should be froth's decision to escalate — hence one hook, not a policy.

---

# What the sketch exposed

## 1. Phase-typed contexts beat a runtime phase check

ADR 0002 has one context type and an always-on runtime assertion. Writing real call
sites, the contexts wanted to be *different types* — `RenderCtx`, `UpdateCtx`,
`ApplyCtx`, `MeasureCtx` — because each phase permits a different operation set. A
`RenderCtx` simply has no `Create`, so "structural change during render" stops being a
runtime error and becomes a compile error.

Strictly better, and not visible from the design discussion. The runtime check still
earns its place for what types can't catch, chiefly a surface stashing a context and
using it later. **Amends ADR 0002.**

## 2. The handler's return value disappeared entirely

It went in two steps. First, once the context accumulates commands, intents, and
structural requests, the `Result` struct we assumed had nothing left in it — `Update`
returned a plain `bool` meaning "did I consume this".

Then writing out the `Surface` interface showed the bool has no consumer either.
Bubbling was what read it, and ADR 0008 replaced bubbling with deepest-first
declarative resolution. Every message now has exactly one correct destination before
delivery: an addressed message goes to its handle, a resolved action goes to the
surface that declared the binding, a raw key goes to the focused surface. Nothing
falls through, so there is nothing to report.

```go
type Surface interface {
    Render(rc *RenderCtx, r Rect) string
    Update(uc *UpdateCtx, msg tea.Msg)
}
```

Worth keeping an eye on: a return value that exists but is ignored invites someone to
invent a meaning for it, so removing it is safer than leaving it in reserve. If a case
appears that genuinely needs fall-through, it should come back as an explicit call —
`uc.Decline()` — rather than a bool everyone has to remember to return correctly.

## 3. Coalescing is the wrong default for producers, and so is batching

We settled "producers coalesce by default". The log tailer shows that's wrong:
coalescing replaces a pending message with a newer one, which for log lines *silently
drops content*. Append-style streams want batching — collapse N pending into one
carrying all of them. Progress-style streams want coalescing, where only the newest
matters.

Defaulting to either silently corrupts the other case, so the honest default is
**neither** — every message delivered, in order — with both offered explicitly.

Writing the merge function out exposed a second thing: batching **changes the delivered
type**. A producer emitting `logLineMsg` with a merge function delivers `logLinesMsg`,
so a surface with a case for the singular silently never matches. That deserves a
diagnostic rather than a doc note, since the failure is silence.

## 4. froth writes almost no widgets, but owns an adapter layer

Adapting bubbles is viable and removes most of the widget cost: text input, list,
table, viewport, spinner, help, and file picker all come from there. froth writes
Button (bubbles has none, since in a single-model world a button is a styled string),
the Scroller, and chrome-aware containers.

What froth does take on is the **adapter layer**, and it is not free. Three
non-uniformities have to be papered over per component:

- **Sizing.** `textinput` has `SetWidth`; `list` and `viewport` have `SetSize`; some
  have neither. No common interface to write one generic adapter against.
- **Focus.** bubbles components track their own `Focused()` while froth holds focus
  authoritatively. The adapter must forward froth's focus into `Focus`/`Blur`, or a
  text input renders focused while receiving no keys.
- **Cursor.** Only some components report one.

That's the maintenance surface, and it tracks bubbles' releases rather than froth's.
Cheaper than writing widgets, but it should be an acknowledged cost rather than a
happy accident.

## 5. Apply needs three entry points, not one

A surface with children has nowhere to create them: the constructor has no `ApplyCtx`
and no handle, and `Update` is too late and in the wrong phase. That gives `Mount` —
one-time Apply init, called once when the surface enters the tree.

Writing real flows exposed two more. A surface restructuring itself in response to
input needs Apply *again*, on demand, with real handles — otherwise every restructure
routes through an ancestor that has no business knowing about it. That is `Reconcile`,
requested with `uc.NeedsReconcile()`: one-time init and as-needed apply, the same
capability at different times.

And froth cancels producers and cascades destruction, but a surface holding something
froth doesn't own had nowhere to release it. That is `Unmount`, leaf-up on destroy. It
should stay rare by design rather than discipline, which is why froth derives a
per-surface context and cancels it on teardown — the most common cleanup, aborting an
in-flight RPC, then needs no teardown code at all.

The shape worth noticing: all three are Apply, differing only in when they run. Nothing
else in the design lets a surface reach that phase, and that is deliberate.

## 6. Several guarantees are best effort, because Go has no `const`

Two places where the design promises something the language won't enforce, both worth
documenting rather than discovering:

`ac.Create(root, newSessionList())` means the caller can retain that pointer and poke
at the surface off-loop, voiding every phase guarantee. Mitigated by there being no
reason to — a child raises intents rather than being read by its parent — but that is
discipline, not enforcement.

The Environment's within-a-frame stability covers the *binding*, not the value behind
it. A surface can store a pointer it found in the Environment and mutate the target,
and nothing will stop it. So the guidance is to put values in the Environment rather
than handles to mutable state; sharing something mutable through it opts out of the
guarantee silently.

A producer's step function is the third. It is usually a method on the surface, so it
can reach every field — the log tailer reaches `l.tailer` that way. Reading an immutable
field is fine and is why the shape is allowed at all, but nothing stops it touching
mutable state from the goroutine, which is the one thing producers must not do. The
document says the goroutine never touches surface state; the API does not enforce it.

None of the three is fixable in Go. All should be stated plainly in the docs, because a
guarantee people believe is total is worse than one they know the edges of.

## 7. Environment keys should carry their type, and contexts should carry the subject

`Env` returning `any` for the caller to assert would be the most frequent call in any
surface's render, so it needs to be typed. Putting the type on the *key* — `Key[T]` —
checks both ends: setting accepts only a `theme.Theme`, reading returns one with no
assertion and no type parameter at the call site.

This originally forced free functions, because Go methods could not have type
parameters. **Go 1.27 lifted that**, so these are ordinary generic methods on the
concrete context types and the API is uniform again.

The second half only became visible once they were methods: the context already knows
which surface it belongs to. `uc.Close()` and `uc.Rect()` were always self-relative, so
`SetEnv`, `Create`, `SetLayout` and `Capture` should be too. That deletes a parameter
from the most common calls and makes the ownership rule structural — setting on an
ancestor or creating a grandchild is not expressible rather than merely forbidden. The
sketch's own bootstrap had been quietly creating a grandchild until this exposed it.

One free function survives: `froth.Env[T](r EnvReader, k *Key[T])`, for a helper called
from more than one phase. Interface methods still cannot have type parameters, so
`EnvReader` carries an untyped lookup with typing layered over it.

## 8. bubbles can't report what it consumed — and fixing that reshaped input routing

The most consequential finding, and it only appeared when the adapter's `Update` had
to return a bool. `Model.Update(msg) (Model, tea.Cmd)` gives no signal about whether
the component consumed the message. Both naive answers are wrong: always `true` and a
focused text input swallows `ctrl-c`; always `false` and the component handles the key
*and* ancestors handle it too.

The obvious fix — match the key against the component's own keymap and forward only
what it wants — does not survive contact. There is no uniform way to enumerate a
component's keys: `list.Model` implements `ShortHelp`/`FullHelp`, `textinput.Model`
exposes a 16-field `KeyMap` struct with no enumerator, and neither offers an `All()`.
Worse, text entry isn't describable as bindings at all — a text input consumes
arbitrary printable runes that appear in no `key.Binding`, so matching against its
keymap would reject ordinary typing.

**How existing Bubble Tea apps handle this: they don't.** The root matches global
bindings explicitly, top-down, before delegating anything; the focused component sits
in a `default:` branch and swallows the remainder. Nothing ever asks a component what
it consumed, because nothing downstream needs to know. Where an app *does* need that
knowledge it hardcodes it — one real codebase carries a comment explaining that
`ctrl+d` is deliberately left unconsumed on a non-empty prompt so it falls through to
the textarea's own delete binding. That is a human, by hand, encoding what the
component won't report.

So the original decision — deliver to the focused surface, bubble root-ward when
unhandled — was wrong, and adapting bubbles is what proved it. Input routing is now
three steps: match declared bindings along the focus chain deepest-first, deliver the
resulting **Action** to the surface that declared it, and only if nothing matched let
the raw key reach the focused surface. See
[ADR 0008](../adr/0008-shortcuts-resolve-to-actions.md).

Three things fall out:

- **The adapter can always return `true`.** Shortcuts are resolved before it sees
  anything, so consuming everything is correct rather than dangerous. The `wants`
  predicate disappeared entirely.
- **Ordinary typing needs one flag, not a predicate.** A surface marked a text sink
  suppresses bare-printable matching while it holds focus; modified chords still
  resolve. One boolean on the focused surface replaces per-component configuration.
- **Adapted components are leafs, necessarily.** Nothing can sit beneath a surface
  that can't say what it consumed, and nothing above can trust its answer.

The cost is that surfaces declare bindings rather than matching keys inline, which is
not how Bubble Tea code is written today. The compensation is that the same
declarations drive the help query and the remap layer, so help can't drift from what
actually fires, and rebinding a key touches no surface code.

## 9. Confirmed working, no changes needed

The intent → open-modal → reply path never names a surface across the boundary in
either direction. Cursor-claim-during-render removes an entire category of stale state.
And the async result landing while a modal was open needed no special handling
anywhere — the case most likely to expose a hole, and it didn't.
