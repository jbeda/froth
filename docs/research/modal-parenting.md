# Modal dialogs: where they sit in the tree, and what that costs

Gathered while slicing M3, to settle whether froth's modals should be parented to the root
or to their opener. Win32 is omitted — `win32-hierarchies.md` covers it and corroborates
the pattern below.

**Core finding up front:** in six of seven systems a modal is **not** a child of its
opener. Every stable system separates three concerns that froth's one parent link fuses —
ownership/lifetime, render position, and blocking scope — and the only system that derives
blocking scope from the parent chain (Qt) is the one whose own documentation undercuts it.

## 1. The web platform

**Position.** `showModal()` never reparents. Only painting changes. CSS Position 4 defines
the top layer as an ordered set whose elements *"do not lay out normally based on their
position in the document; instead they generate boxes as if they were siblings of the root
element"*, with the stated rationale that this *"ensures that elements in the top layer
cannot be clipped by anything in the document, or obscured by anything except elements
later in the top layer"*
([drafts.csswg.org/css-position-4/#top-layer](https://drafts.csswg.org/css-position-4/#top-layer)).
The containing block becomes the initial containing block, deliberately ignoring any
transformed or filtered ancestor — which is why `position: fixed` is *not* the same
mechanism. MDN: the top layer *"is an internal browser concept and cannot be directly
manipulated from code"*
([MDN Glossary](https://developer.mozilla.org/en-US/docs/Glossary/Top_layer)).

Because only paint detaches, inheritance, selector matching and event dispatch still follow
the real tree.

**Lifetime** is coupled to DOM connectedness, but explicitly: `showModal()` throws
`InvalidStateError` on a disconnected element, and removal evicts from the top layer
*immediately*
([HTML spec](https://html.spec.whatwg.org/multipage/interactive-elements.html#dom-dialog-showmodal)).
Defined synchronous teardown, not a dangling reference.

**Blocking** is a wholly separate mechanism. While blocked by a modal dialog, *"every node
that is connected to document, with the exception of the subject element and its flat tree
descendants, must become inert"*
([HTML spec](https://html.spec.whatwg.org/multipage/interaction.html#inert)). Inertness is
enumerated facility by facility — hit testing acts as `pointer-events: none`, nodes are
hidden from accessibility APIs, selection acts as `user-select: none`, find-in-page skips
them. Scope is *everything connected to the document*, not a subtree.

**Gotchas.** Exit animations needed a new `overlay` property plus
`transition-behavior: allow-discrete`, because top-layer removal is otherwise instantaneous
([MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/overlay)). `::backdrop` inheritance
was reversed by the CSSWG ([csswg-drafts#7845](https://github.com/w3c/csswg-drafts/issues/7845)).
There is no API to enumerate or reorder the top layer. And blocking's blunt scope makes
unrelated top-layer content inert, since only the dialog's own descendants are exempt.

## 2. GTK 4 — the maximally decoupled design

Three fully orthogonal knobs, none implying another:

- **`transient_for`** — stacking and placement only. *"This allows window managers to keep
  the dialog on top of the main window, or center the dialog over the main window."*
  ([docs.gtk.org](https://docs.gtk.org/gtk4/method.Window.set_transient_for.html))
- **`set_modal`** — blocking, application-wide.
  ([docs.gtk.org](https://docs.gtk.org/gtk4/method.Window.set_modal.html))
- **`destroy-with-parent`** — lifetime, and it **defaults to `FALSE`**.
  ([docs.gtk.org](https://docs.gtk.org/gtk4/property.Window.destroy-with-parent.html))

A dialog is a `GtkWindow`, which implements `GtkRoot`, so it is not in the opener's widget
tree at all.

`gtk_dialog_run()` was **removed** in GTK 4: *"Nested main loops present re-entrancy issues
and other hard to debug issues when coupled with other event sources."*
([migrating-3to4](https://docs.gtk.org/gtk4/migrating-3to4.html)). Emmanuele Bassi, more
bluntly: *"the introduction of gtk_dialog_run() was, in retrospect, a mistake"*
([GNOME Discourse](https://discourse.gnome.org/t/how-should-i-replace-a-gtk-dialog-run-in-gtk-4/3501)).

## 3. Qt — the one system that derives blocking from the parent chain

A `QDialog` is *"always a top-level widget, but if it has a parent, its default location is
centered on top of the parent"* ([doc.qt.io](https://doc.qt.io/qt-6/qdialog.html)). The
parent survives as ownership plus placement, and QObject ownership cascades: *"it's added
to the parent's children() list, and is deleted when the parent is"*
([Object Trees](https://doc.qt.io/qt-6/objecttrees.html)) — so a **grandparent's** deletion
reaches an open modal dialog.

`Qt::WindowModal` *"blocks input to its parent window, all grandparent windows, and all
siblings of its parent and grandparent windows"*, versus `Qt::ApplicationModal` which
blocks everything ([Qt::WindowModality](https://doc.qt.io/qt-6/qt.html#WindowModality-enum)).
*[The "grandparent … siblings" continuation was corroborated across two passes but not
re-fetched verbatim — high confidence, not doubly sourced.]*

Qt's own docs steer away from the synchronous path, and name the hazard: *"Avoid using this
function; instead, use open(). Unlike exec(), open() is asynchronous, and does not spin an
additional event loop. This prevents a series of dangerous bugs from happening (e.g.
deleting the dialog's parent while the dialog is open via exec())."* And `exec()` silently
forces `ApplicationModal`, overriding a configured `WindowModal`.

## 4. macOS / AppKit

A sheet is its own `NSWindow` linked by a **dedicated relation separate from**
`addChildWindow:`/`childWindows`: *"The window object in this property refers to the window
to which the sheet is logically attached, regardless of appearance"*
([NSWindow.sheetParent](https://developer.apple.com/documentation/appkit/nswindow/1419052-sheetparent)).
No view-tree relationship at all, and the relation is lifecycle-scoped to the sheet's
presentation.

Two different blocking mechanisms, and Apple distinguishes them. Sheets: *"While the sheet
remains visible, most events targeted at the receiver are prohibited. The runloop does not
enter any special mode to accomplish this."*
([beginSheet](https://developer.apple.com/documentation/appkit/nswindow/beginsheet(_:completionhandler:))).
App-modal `runModal(for:)` is the opposite — it starves timers and other event sources.

A second `beginSheet` on the same parent **queues** rather than nesting. What happens when
a parent with an attached sheet closes is **[UNVERIFIED — not stated in reachable current
Apple docs]**.

## 5. Flutter — the closest analogue to froth's design

`showDialog` pushes a route onto a `Navigator`, rendered into an `Overlay`. The dialog's
element sits under the Overlay, not the caller: *"The widget returned by the `builder` does
not share a context with the location that showDialog is originally called from."* And by
default *"`useRootNavigator` … is `true`"*
([showDialog](https://api.flutter.dev/flutter/material/showDialog.html)).

The lifetime sentence is the decisive one: *"The `context` argument is used to look up the
Navigator and Theme for the dialog. It is only used when the method is called. **Its
corresponding widget can be safely removed from the tree before the dialog is closed.**"*

Blocking is an explicitly inserted widget — `ModalBarrier`, *"a widget that prevents the
user from interacting with widgets behind itself"*.

**The documented cost is ambient resolution, and it is the most-reported problem in the
set.** Because the dialog hangs off the root Overlay, `InheritedWidget` lookups resolve
against the Overlay's ancestors, not the caller's: Theme from the wrong ancestor, Provider
not found, bloc null on one platform and not another —
[#23808](https://github.com/flutter/flutter/issues/23808),
[#28505](https://github.com/flutter/flutter/issues/28505),
[#87018](https://github.com/flutter/flutter/issues/87018),
[provider#546](https://github.com/rrousselGit/provider/issues/546). Flutter's docs
pre-empt the complaint in prose and it still generates issues years later.

## 6. React — the inverse of everyone else

A portal moves the DOM node and nothing else: *"A portal only changes the physical
placement of the DOM node. In every other way, the JSX you render into a portal acts as a
child node of the React component that renders it."* And *"Events from portals propagate
according to the React tree rather than the DOM tree"*
([react.dev](https://react.dev/reference/react-dom/createPortal)). Render moves out,
ownership stays in. React supplies **no** blocking mechanism, which is why its own docs
point at the WAI-ARIA modal authoring practices instead.

## 7. Android

A `Dialog` owns its own `Window` added to `WindowManager` with the host Activity's token;
it is not a view in the Activity's hierarchy
([Android dialogs guide](https://developer.android.com/guide/fragments/dialogs)).
`DialogFragment` lives in the `FragmentManager` tree, and Google moved ownership there
precisely to survive configuration changes — a bare `Dialog` leaks (`WindowLeaked`) or
throws `BadTokenException` when the host token is invalidated. *[The token mechanism is
documented; the exception-message rationale is secondary-source corroborated, not
primary-quoted.]*

Blocking is a window-layer flag — the **absence** of `FLAG_NOT_TOUCH_MODAL` is what makes a
dialog touch-modal. Nothing to do with view-tree position.

## Synthesis

**The dominant pattern.** Nobody renders a modal inside the opener's subtree. Six of seven
place it outside any opener-rooted tree entirely; React moves the render out and keeps
ownership in.

**Blocking is never a consequence of tree position — except in Qt.** That is the strongest
signal in the survey: the one system that derived blocking scope from the parent link is
the one with a documented trapdoor (`exec()` overriding the configured scope).

**Lifetime coupling is closer to even, and the lesson is about observability.** The web and
React couple lifetime to position; Flutter, GTK and AppKit decouple it; Android decoupled
after starting coupled; Qt couples and warns. What separates the workable couplings from
Qt's is that their teardown is **defined and immediate** — `showModal()` throws rather than
half-working, removal evicts synchronously.

**Why Flutter and Android decoupled: framework churn.** Flutter's opener may be a recycled
list item; Android's may be an Activity destroyed by rotation. In both, the opener's
disappearance is bookkeeping unrelated to the user's question. A system without churn —
where a node is destroyed only by deliberate application decision — does not inherit that
motivation.

**Enumeration.** Every windowing toolkit provides application-level enumeration of
top-level windows — `gtk_application_get_windows`, `QApplication::topLevelWidgets`,
`NSApp.windows` — **distinct from** parent-tree traversal. A modal is findable from the
application, never by an arbitrary ancestor walking down. The web is the outlier:
`querySelectorAll` from any ancestor still finds a modal wherever it sits, which is the
price of moving only paint.

## Bearing on froth

1. **Root-parenting matches the strongest precedent.** froth's design is Flutter's almost
   exactly — route owned by a top-level owner, result delivered by a channel that is not
   the tree.
2. **The lifetime decision went the other way anyway**, because froth has no churn. See
   [ADR 0006](../adr/0006-one-parent-relation.md)'s amendment.
3. **Expect ambient resolution to be the complaint, not lifetime.** Flutter's issue tracker
   is the evidence. The mitigation shape to steal is `useRootNavigator`: an opener-chosen
   parent decided at create time, which is still one link fixed at creation rather than a
   second graph.
4. **Qt is the cautionary case** for deriving modality scope from the parent link. See
   [ADR 0010](../adr/0010-presentation-is-metadata-authority-is-the-link.md)'s amendment.

## Appendix: how the application-level object is reached

Asked separately, because froth gates access more tightly than these do. Summarised here;
the full study, with per-framework detail and citations, is in
[`app-object-access.md`](./app-object-access.md). Ranked most-ambient to most-scoped:

| Framework | Object | Reached by | Gating |
|---|---|---|---|
| Web | `window` / `document` | the global object | none |
| Qt | `QApplication` | `qApp` / `QCoreApplication::instance()` | none (main-thread only) |
| AppKit | `NSApplication` | `NSApp` / `.shared` | none (main-thread only) |
| Flutter | `WidgetsBinding.instance` | static getter | none |
| GTK 4 | `GtkApplication` | a method on something you hold — **no public global** | must hold a reference |
| Android | `Context` | must already hold one — **no platform global** | must hold a reference |
| Flutter | `Navigator` / `Overlay` | `.of(context)` — requires a `BuildContext` | tree-position token |

**No framework treats reaching the app object as a capability.** The two that avoid a
global — GTK 4 and Android — are the most recent designs, so the trend is away from
ambient, but neither gates access. Flutter's `BuildContext` requirement is the only
token-scoped one, and the evidence says it is a **mechanical byproduct**: the docs describe
`BuildContext` as *"a handle to the location of a widget in the widget tree"* and frame
`.of(context)` as walking up from that position. No design rationale framing it as access
control was found. *[Unverified: Apple's and Android's JS-rendered doc pages did not yield full body text, so
`NSApplication.shared`'s exact wording and Android's Context-leak wording are reported from
secondary corroboration rather than primary quotation. Re-fetch before either goes into an
ADR.]*
