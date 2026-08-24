# Who may write an ambient binding, and can a parent target one child?

Gathered to settle whether froth's Environment should keep both write forms — a surface
setting a value on itself, and a surface setting a value on an immediate child. Only
**write authority** is in scope. Resolution is settled (root-ward from the reader, nearest
binding wins) and is not re-litigated here.

**Core finding up front:** in every declarative system surveyed, "the parent writes the
child's binding" is not a separate authority — it *is* wrapping, and the wrapper is an
ordinary node the parent renders. Only the two systems with long-lived imperative trees
(Android, Qt) have a real per-child write, and **both of them are where the documented
regret lives**. The regret is not about two writers racing; it is about a parent's write
silently defeating a component's own internal configuration.

---

## 1. SwiftUI — the modifier is a node, not an attribute

Scope of a write is stated twice in matching language. `environment(_:_:)`: *"This modifier
affects the given view, as well as that view's descendant views. It has no effect outside
the view hierarchy on which you call it."*
([developer.apple.com](https://developer.apple.com/documentation/SwiftUI/View/environment(_:_:))).
`EnvironmentValues`: *"The value that you set affects the environment for the view that you
modify — including its descendants in the view hierarchy — but only up to the point where
you apply a different environment modifier."*
([developer.apple.com](https://developer.apple.com/documentation/swiftui/environmentvalues)).

The mechanism is wrapping, despite reading like an attribute at the use site. `ViewModifier`
is *"A modifier that you apply to a view or another view modifier, producing a different
version of the original value"*, and `modifier(_:)` returns `ModifiedContent<View, Modifier>`
([developer.apple.com](https://developer.apple.com/documentation/swiftui/viewmodifier)). So
`.environment` chained onto a child in the parent's body **is** the React trick with
different syntax: it inserts a node above that child. It looks like froth's `SetEnvOn` and
is not.

Reading is one-directional by construction. `@Environment` is *"a property wrapper you can
use to read — but not set — an environment value"*, and there is no upward-write analogue.
SwiftUI's child-to-ancestor channel is an entirely separate feature, `PreferenceKey` — *"A
named value produced by a view"*, where *"A view with multiple children automatically
combines its values for a given preference into a single value visible to its ancestors"*
([developer.apple.com](https://developer.apple.com/documentation/swiftui/preferencekey)).
That a separate mechanism had to be built for the other direction is the strongest available
evidence that ancestor-writes through the environment were never on the table.

*[Two things were not found in primary sources and are inference from composition, not Apple
statements: (a) whether a view reads its own `.environment` write — structurally it cannot,
because `body`'s `@Environment` reads resolve against the incoming environment before the
returned content is wrapped; (b) how `.environment(k,a).environment(k,b)` resolves — chaining
makes `a` the inner wrapper, so the wrapped content sees `a`. React documents the equivalent
of (a) explicitly (below), which corroborates but does not source it.]*

## 2. React — a provider is a node, and the docs say so about the failure case

*"Wrap your components into a context provider to specify the value of this context for all
components inside"*
([react.dev](https://react.dev/learn/passing-data-deeply-with-context)). Nearest wins:
`useContext` *"returns the context value for the calling component. It is determined as the
`value` passed to the closest `SomeContext` above the calling component in the tree."*
([react.dev](https://react.dev/reference/react/useContext)).

React documents the self-write question that SwiftUI leaves to inference: *"`useContext()`
call in a component is not affected by providers returned from the **same** component. The
corresponding `<Context>` needs to be **above** the component doing the `useContext()`
call."* — with a flagged failing example
([react.dev](https://react.dev/reference/react/useContext)). A component can only ever write
for its descendants, never for itself.

There is no non-wrapping write. React 19's bare `<Context value={...}>` is terser syntax over
the same node. *[No react.dev sentence was found asserting the two forms are structurally
identical; inferred from consistent usage in current docs.]*

The interesting part for froth is what react.dev recommends **before** reaching for context
at all: *"Just because you need to pass some props several levels deep doesn't mean you
should put that information into context."* Its ordered alternatives are (1) *"Start by
passing props... it makes it very clear which components use which data!"*, (2) *"Extract
components and pass JSX as `children` to them"*, and only then (3) *"consider context"*
([react.dev](https://react.dev/learn/passing-data-deeply-with-context)). Alternative (2) —
composition — is precisely the answer to "I want one child different from its siblings",
and it is a structural answer, not a write.

## 3. Flutter — writing is constructing, because widgets cannot be mutated

`InheritedWidget` is the *"Base class for widgets that efficiently propagate information down
the tree"*
([api.flutter.dev](https://api.flutter.dev/flutter/widgets/InheritedWidget-class.html));
`Theme` *"applies a theme to descendant widgets"* and `Theme.of` returns *"the data from the
closest Theme instance that encloses the given context"*
([api.flutter.dev](https://api.flutter.dev/flutter/material/Theme-class.html)). Nearest wins,
via `dependOnInheritedWidgetOfExactType`, which *"returns the nearest widget of the given
type `T`"* and registers the dependency
([api.flutter.dev](https://api.flutter.dev/flutter/widgets/BuildContext/dependOnInheritedWidgetOfExactType.html)).

The write-authority question is closed by immutability rather than by policy. *"A widget is
an immutable description of part of a user interface... Widgets themselves have no mutable
state (all their fields must be final)"*
([api.flutter.dev](https://api.flutter.dev/flutter/widgets/Widget-class.html)). There is no
in-place set at all — the only write is constructing a new `Theme`/`InheritedWidget` around a
subtree. Update propagation is gated by `updateShouldNotify`, which implementers *"must
override... to determine when dependent widgets should be rebuilt"*, and reaches only
widgets that registered a dependency.

Per-child scoping is again wrapping: `Theme(data: ..., child: thatWidget)`. No addressed
per-child write exists.

`provider`'s documented hazards are about *timing and depth*, not authority: modifying a
`ChangeNotifier` from a descendant while the tree is building is called out as a common
error, and deep `MultiProvider` nesting is documented as risking `StackOverflowError`
([pub.dev/packages/provider](https://pub.dev/packages/provider)).

## 4. Vue — component-level provide, wrapper for narrowing

*"A parent component can serve as a dependency provider for all its descendants. Any
component in the descendant tree, regardless of how deep it is, can inject dependencies
provided by components up in its parent chain."* Nearest wins: *"If multiple parents provide
data with the same key, inject will resolve to the value from the closest parent in
component's parent chain."*
([vuejs.org](https://vuejs.org/guide/components/provide-inject)).

No per-child mechanism was found; narrowing to one child means interposing a component.
`app.provide()` exists as the opposite extreme — app-wide, documented as mainly for plugins.

Vue is the one system that offers explicit *discipline* on who mutates: *"When using reactive
provide / inject values, it is recommended to keep any mutations to reactive state inside of
the provider whenever possible. This ensures that the provided state and its possible
mutations are co-located in the same component, making it easier to maintain in the future."*
The sanctioned pattern is providing the value together with a mutator function, and
`readonly()` is offered to block injector-side mutation
([vuejs.org](https://vuejs.org/guide/components/provide-inject)). This is a warning about
*two writers for one value* — the injector and the provider — and its answer is to keep
writes at one site.

## 5. Angular — the one declarative system with a genuine per-child write

Static form is per-component-type, not per-instance. `providers` *"Configures the injector of
this directive or component with a token that maps to a provider of a dependency"*;
`viewProviders` *"Defines the set of injectable objects that are visible to its view DOM
children"*, and content projected through `<ng-content>` cannot see them
([angular.dev/api/core/Component](https://angular.dev/api/core/Component),
[hierarchical DI guide](https://angular.dev/guide/di/hierarchical-dependency-injection)).
Resolution walks up element injectors to the environment injector, and a local provider stops
the walk: *"If, for example, a provider is registered locally in the component that needs a
service, Angular doesn't look for another provider of the same service."*

The per-child write exists, but only for **dynamically created** children, and it is handed
over at creation:

- `ViewContainerRef.createComponent(componentType, {injector})` — `injector` is *"the injector
  to use as the parent for the new component"*
  ([angular.dev](https://angular.dev/api/core/ViewContainerRef)).
- `NgComponentOutlet`'s `ngComponentOutletInjector` — *"Optional custom `Injector` that will
  be used as parent for the Component. Defaults to the injector of the current view
  container."* ([angular.dev](https://angular.dev/api/common/NgComponentOutlet)).

That is the closest analogue found to a parent addressing one child — and note the shape: the
parent supplies it **as a creation parameter**, not as a later imperative call on a live
child. No template syntax for per-element providers was found. *[No documented warning against
provider overuse was found on the pages fetched.]*

## 6. CSS — unrestricted write authority, and a stack of mechanisms built to contain it

The authority model is maximally open. A declaration applies to an element if *"It belongs to
a style sheet that currently applies to this document... It belongs to a style rule whose
selector matches the element"*
([drafts.csswg.org/css-cascade](https://drafts.csswg.org/css-cascade/) §5). There is no notion
of an element's declarations being owned by anyone. Custom properties are `Inherited: yes`
([drafts.csswg.org/css-variables](https://drafts.csswg.org/css-variables/)), which is what
makes them the ambient channel.

Conflicts are resolved by a six-level sort — origin and importance, encapsulation context,
element-attached styles, layers, specificity, order of appearance
([drafts.csswg.org/css-cascade-5](https://drafts.csswg.org/css-cascade-5/) §6.1). Level 2 is
the one that answers this survey's question directly: *"When comparing two declarations...
from different encapsulation contexts, then for normal rules the declaration from the outer
context wins, and for important rules the declaration from the inner context wins."* CSS's
default answer to parent-versus-child is **the outer writer wins**, with `!important`
reversing it.

Practitioners treat cross-boundary writing as a problem, and the record is in the mechanisms
built to stop it:

- Shadow DOM blocks *selectors* at the boundary — *"The page CSS does not affect nodes inside
  the shadow DOM"* — while deliberately leaving inheritance and custom properties as the
  sanctioned channel: *"the component would advertise that it uses certain custom properties
  to style its internals, and the outer page could then set those properties as it wished on
  the shadow host, letting inheritance push the values down to where they were needed"*
  ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Web_components/Using_shadow_DOM),
  [drafts.csswg.org/css-shadow-1](https://drafts.csswg.org/css-shadow-1/) §5.1).
- `::part()` exists because full blocking was too strong: *"this styling barrier also makes it
  harder for a page to interact with its components when it actually wants to do so"* (same
  spec section).
- `/deep/` was removed for being too *weak* a boundary: *"it exposed too much of a component's
  internal structure to scrutiny, defeating some of the encapsulation benefits that using
  Shadow DOM brings"* (same section). Chrome's removal notice: *"The primary concern with
  shadow-piercing selectors is that they violate encapsulation and create situations where a
  component can no longer change its internal implementation"*
  ([Chrome for Developers](https://developer.chrome.com/blog/remove-shadow-piercing)).
- `@scope` limits *selector* reach and explicitly does **not** contain inherited values:
  *"while `@scope` allows you to isolate the application of selectors to specific DOM
  subtrees, it does not completely isolate the applied styles to within those subtrees. This
  is most noticeable with inheritance"*
  ([MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/@scope)).

Honest reading: **the cascade specs express no regret about open write authority.** They
define it as the model and give it a total order. The encapsulation mechanisms are framed as
opt-in tools, and the only regret-shaped language found — *"too powerful for its own good"* —
is aimed at `/deep/`, a piercing selector, not at inheritance.

## 7. Go `context.Context` — the callee cannot write anything at all

*"The chain of function calls between them must propagate the Context, optionally replacing
it with a derived Context"*; `WithValue` *"returns a derived context that points to the parent
Context"* ([pkg.go.dev/context](https://pkg.go.dev/context)). Lookup is a walk up the chain,
nearest binding first — `valueCtx` holds one key/value and a parent, and `Value` checks itself
then recurses outward
([src/context/context.go](https://github.com/golang/go/blob/master/src/context/context.go)).

There is exactly one writer per binding and it is the caller, enforced structurally rather
than by policy: `Context` is an interface with no setter, `valueCtx.val` is assigned at
construction, and a callee holds a copy of an interface value. Deriving affects only what the
callee passes further down. No write-back path exists in the type.

The scope guidance is the part worth carrying: *"Use context values only for request-scoped
data that transits processes and API boundaries, not for passing optional parameters to
functions."* Also *"Do not store Contexts inside a struct type"* and *"Users of WithValue
should define their own types for keys"* (same page). *[No Go-team primary source expressing
regret about `WithValue` was found — the blog post and the issues checked are about
implementation details. The restrictive usage sentence reads as a guardrail set at
introduction, not retrospective regret. Third-party criticism exists but is not attributable
to the Go team.]*

## 8. Android — a parent's declaration setting one child's theme, and the regret to go with it

This is the closest structural match to froth's `SetEnvOn`. *"Beginning with Android 5.0 (API
level 21)... you can also specify the `android:theme` attribute to a view in your layout file.
This modifies the theme for that view and any child views, which is useful for altering theme
color palettes in a specific portion of your interface."*
([developer.android.com](https://developer.android.com/develop/ui/views/theming/themes)).

The subtree scope is mechanical rather than designed: `LayoutInflater.createViewFromTag` reads
the attribute and does `context = new ContextThemeWrapper(context, themeResId)`, then threads
that `context` into the recursive inflation of that view and its children
([LayoutInflater.java](https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/view/LayoutInflater.java)).
So even here, the "attribute" is a wrapper — of the *context object*, not of the view tree.

A node can also write for itself, but only before it has children: `Context.setTheme` — *"Set
the base theme for this context. Note that this should be called before any views are
instantiated in the Context (for example before calling `Activity#setContentView` or
`LayoutInflater#inflate`)."*
([Context.java](https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/content/Context.java)).

Two writers for the same node's binding is therefore possible, and the merge is per-attribute,
not whole-object. `ContextThemeWrapper(Context, int)`: *"The specified theme will be applied
on top of the base context's theme. Any attributes not explicitly defined in the theme
identified by themeResId will retain their original values."* The `Resources.Theme` overload
instead *"will completely replace the base context's theme"*
([ContextThemeWrapper.java](https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/view/ContextThemeWrapper.java)).
`Resources.Theme.applyStyle(int, boolean force)` makes the precedence explicit: *"If false,
only values that are not already defined in the theme will be copied... otherwise, if any of
the style's attributes are already defined in the theme, the current values in the theme will
be overwritten."*
([Resources.java](https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/content/res/Resources.java)).

**The documented regret.** `MaterialThemeOverlay` exists specifically because parent-set child
themes clobber a component's own theming: it *"provides protection against losing the
ThemeOverlay by clients who set `android:theme` or `app:theme`. If `android:theme` or
`app:theme` is specified by the client, any attributes defined there will take precedence over
attributes defined in `materialThemeOverlay`."*
([developer.android.com](https://developer.android.com/reference/com/google/android/material/theme/overlay/MaterialThemeOverlay)).
A whole utility class had to be shipped so components could defensively re-wrap their own
context. Note the shape of the failure precisely: it is **not** a race between two writers. It
is that the outer writer wins and the component's own configuration is silently lost.

*[No warning attached to `android:theme` on child views was found on the theming guide itself,
which presents the feature plainly. The regret is in the Material Components layer, not the
platform docs.]*

## 9. Qt — anyone may write any widget, and the resolve mask is undocumented

*"QWidget propagates explicit palette roles from parent to child. If you assign a brush or
color to a specific role on a palette and assign that palette to a widget, that role will
propagate to all the widget's children, overriding any system defaults for that role."* The
`font` property doc is word-for-word parallel
([doc.qt.io/qt-6/qwidget.html](https://doc.qt.io/qt-6/qwidget.html#palette-prop)).

`setPalette` is a plain public member function. Nothing gates who calls it — parent, child, or
unrelated code holding a pointer. The precedence rule is per-role union, documented on
`QPalette::resolve`: *"Returns a new QPalette that is a union of this instance and other. Color
roles set in this instance take precedence. Roles that are not set in this instance will be
taken from other."*
([doc.qt.io](https://doc.qt.io/qt-6/qpalette.html#resolve)). Unlike CSS and Android, Qt's
answer to parent-versus-child is **the inner (explicitly-set) writer wins**, and the widget
then stops receiving ambient updates for those roles.

The flags implementing that — `WA_SetPalette = 36`, `WA_SetFont = 37`, `WA_SetStyle = 86` in
[qnamespace.h](https://code.qt.io/cgit/qt/qtbase.git/tree/src/corelib/global/qnamespace.h) —
carry **no comment in the source**, and the enum table's descriptions could not be retrieved
from doc.qt.io. That is itself a finding: the mechanism deciding write precedence in a system
where anyone may write is effectively undocumented internal machinery rather than a public
contract. *[The exact "where the respective brush or font hasn't been explicitly set"
qualifier could not be re-fetched verbatim; the flags' existence and role are confirmed from
source, the precise doc wording is unverified.]*

**The documented warning is blunt** and appears on both `QWidget::setPalette` and
`QApplication::palette`: *"Do not use this function in conjunction with Qt Style Sheets."*
Also: *"Some styles do not use the palette for all drawing, for instance, if they make use of
native theme engines. This is the case for the Windows Vista and macOS styles."*
([doc.qt.io/qt-6/qwidget.html](https://doc.qt.io/qt-6/qwidget.html#setPalette)) — so a write
may simply not be honoured. Qt ended up with two non-composing propagation systems: stylesheet
changes *"are propagated to all widgets matching the style sheet once, at the time the change
is made"*
([doc.qt.io](https://doc.qt.io/qt-6/stylesheet-syntax.html)), versus imperative writes that
propagate to existing and future children through the resolve mask. *[A compatibility flag
`AA_UseStyleSheetPropagationInWidgetStyles` was reported by search but not fetched verbatim —
unverified.]*

---

## Cross-cutting findings

**1. Every declarative system answers "one child different" with structure, and none has an
addressed per-child write.** SwiftUI, React, Flutter and Vue all narrow scope by placing a
node above the target. The syntax differs enormously — a chained modifier at the use site
looks nothing like `<Provider>` wrapping — but `ModifiedContent` shows SwiftUI's is a node
too. The question "may a parent write a child's binding" therefore **does not arise** in these
systems, not because it was decided against, but because a write *is* a node placement and
nodes are free.

**2. The one declarative exception proves the rule about timing.** Angular's genuine per-child
write (`createComponent({injector})`, `ngComponentOutletInjector`) applies only to
dynamically-created children and is supplied **at creation**, alongside the component type. It
is a constructor parameter in all but name.

**3. Two writers for the same node's binding is common, and every system that has it resolves
it by a static rule, never by recency.** CSS: the outer context wins for normal rules, inner
for `!important`. Qt: the inner explicit set wins, per-role. Android: additive union, with the
outer `android:theme` winning where both define an attribute, and a `force` flag to invert it.
Notably **none of these is "last write wins"** — every one of them is positional or
declarative, decidable without knowing execution order.

**4. Where a parent can reach into a child, the documented failure is silent clobbering, not
contention.** `MaterialThemeOverlay` exists because a client's `android:theme` on a child
silently defeats the component's own overlay. Qt's warning is that `setPalette` and
stylesheets don't compose and the write may not be honoured. Both are one-writer-wins-and-the-
other-vanishes, not a race. This is the honest shape of the hazard.

**5. The regret found is about boundaries, not about counting writers.** CSS built Shadow DOM,
`::part`, `@scope` and CSS Modules; the CSSWG killed `/deep/` for *"violating encapsulation"*.
The consistent complaint is that an outside writer can reach a component's internals and
thereby freeze its implementation. It is never "two writers is confusing".

**6. Two systems document a discipline, and both put writes at one site.** Vue: keep mutations
*"inside of the provider"* so state and its mutations are co-located. Go: contexts are derived,
never mutated, so exactly one party ever writes a given binding. React's steer is stronger
still — pass props first, compose second, context third.

**7. Nobody lets a node write upward.** Unanimous. SwiftUI needed a separate feature
(`PreferenceKey`) for the other direction; Go has no write-back path in the type; Vue's
`readonly()` blocks the injector; React documents that a component cannot even affect its own
read. froth's decision that setting on an ancestor is not expressible has no dissenters.

**8. Reading your own write is documented as impossible where it is documented at all.**
React says so outright. SwiftUI's structure implies it. This differs from froth, where a
surface's write on itself covers its own subtree *including itself*, because resolution starts
at the reader rather than above it. That is not a wrapping system, so the constraint does not
follow.

---

## What transfers to froth, and what does not

**Does not transfer: the claim that per-child writes are unnecessary.** Four of the nine
systems avoid an addressed per-child write only because wrapping is available and cheap. ADR
0009 forbids exactly that. Their silence on the question is not evidence against `SetEnvOn`;
it is evidence that they solved the *use case* another way. The use case itself — one child
different from its siblings — is universal and every system serves it.

**Transfers: the per-child write, where it exists, is a creation-time parameter.** Angular's
per-child injector goes in at `createComponent`. Android's `android:theme` is read during
inflation, before the view exists. Qt is the only system where a per-child ambient write lands
on a live, already-built object — and Qt is the one with the blunt documented warning and the
undocumented precedence machinery. This is the sharpest signal in the survey, and it bears
directly on froth's writes being *immediate* rather than at create time.

**Transfers: conflict resolution should be positional, not temporal.** No surveyed system
resolves competing ambient writes by recency. CSS uses context-then-specificity-then-order; Qt
uses explicit-set-wins per role; Android uses defined-wins with an explicit override flag. All
are decidable by inspection. If froth allows both a surface and its parent to write the same
surface's binding, "whoever ran last" would be the only unprecedented answer among the nine.
*(froth's frame-stable Apply-only writes already remove most of the temporal ambiguity within
a frame; across frames the question stands.)*

**On whether two writers per binding is a real hazard: partly real, and not in the way the
concern is phrased.** The evidence found is one-sided in a useful way.

- *No system documents a problem with two writers contending.* Not CSS, which has the most
  writers per element of anything surveyed and treats it as the model rather than a defect.
  Not Android, not Qt. Searched deliberately; not found.
- *Two systems document the adjacent problem, which is real:* a parent's write silently
  defeating the child's own configuration, with the child unable to see it happened or to
  defend against it except by re-wrapping (`MaterialThemeOverlay`) or by having already set
  the value explicitly (Qt's resolve mask). Both are cases where the same binding has two
  writers and the loser gets no signal.
- *The strongest structural evidence against two writers is Go's,* and it is by construction
  rather than by argument: exactly one party ever writes a binding, and the reason is that
  there is no write-back path in the type at all. Vue reaches the same place by
  recommendation — *"keep any mutations to reactive state inside of the provider"* — which is
  a documented preference for one writer per value.

So: contention is a theoretical concern with no supporting evidence in the record. **Silent
override is a documented, named, twice-independently-encountered failure**, and it is exactly
what "both a surface and its parent may write this surface's binding" enables. The question
worth carrying forward is not "how do two writers race" but "when the parent's write and the
surface's own write disagree, which one loses, and does the loser find out".

**Transfers, on "explicit over magic": the constructor-parameter alternative is what these
systems actually recommend.** react.dev's ordered guidance puts props first and says so in
terms of legibility — *"it makes it very clear which components use which data!"*. Go's
`WithValue` doc restricts contexts to *"request-scoped data that transits processes and API
boundaries, not for passing optional parameters to functions."* Both are already the substance
of the sketch's own "When to use it" test, and the survey corroborates it rather than adding
to it.

**Unverified, flagged before any of this reaches an ADR:** SwiftUI's self-write behaviour and
chained-modifier resolution are inferred from `ModifiedContent` composition, not quoted from
Apple; Qt's `WA_SetPalette` semantics are confirmed as flags in source but the doc prose could
not be retrieved and carries no source comment; `AA_UseStyleSheetPropagationInWidgetStyles`
was reported by search only; no Go-team statement of regret about `WithValue` was found; no
Angular warning about provider overuse was found.

**Incidental, noticed while reading:** `kitchen-sink-sketch.md` line 520 has `appRoot.Apply`
calling `ac.SetEnvOn(ac.Root(), ThemeKey, ...)` — a surface using the child-targeting form on
itself, in an example that also still uses the removed `Intent`. Stale against ADR 0004's
amendment either way.
