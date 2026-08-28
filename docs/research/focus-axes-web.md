# Focus axes on the web platform

Prior-art survey for froth's decision about walkable / focusable-by-name / clickable.
Sources are the HTML Standard (WHATWG), CSS Basic User Interface Level 4, CSSOM View,
the WAI-ARIA Authoring Practices Guide, and Open UI + WHATWG PR material for
`focusgroup`. Retrieved 2026-08-27. MDN is not relied on anywhere below.

---

## A. The focusability trichotomy

### A.1 The web has exactly three, and says so

froth's hypothesis is the platform's own conclusion. WHATWG HTML issue #4607
("Focus meta-bug", the issue that drove the current spec text) opens with:

> There appear to be three types of focusability:
> - Programmatically focusable (`.focus()`, `autofocus=""`)
> - Click focusable (clicking with a mouse, maybe/hopefully some AT overrides?)
> - Sequentially focusable (tabbing)
>
> Only the first one is script-observable or testable. The others are based on user
> interaction.

<https://github.com/whatwg/html/issues/4607>

The same issue tabulates real combinations across platforms, e.g. `<span tabindex="-1">`
on Windows Firefox/Chrome is programmatically yes / click yes / sequentially no; a `<div>`
with overflowing contents on Windows Chrome Canary is programmatically yes / click **no** /
sequentially **yes**; `<input type=checkbox>` on macOS Safari is programmatically yes /
click **no** / sequentially no. All eight-ish corners are populated in the wild.

### A.2 The normative definitions

**Focusable area** — HTML §6.6.2 Data model
(<https://html.spec.whatwg.org/multipage/interaction.html#focusable-area>):

> The term focusable area is used to refer to regions of the interface that can further
> become the target of such keyboard input. Focusable areas can be elements, parts of
> elements, or other regions managed by the user agent.

Elements qualify as focusable areas when they meet **all** of:

> - the element's tabindex value is non-null, or the element is determined by the user
>   agent to be focusable;
> - the element is either not a shadow host, or has a shadow root whose delegates focus is
>   false;
> - the element is not actually disabled;
> - the element is not inert;
> - the element is either being rendered, delegating its rendering to its children, or
>   being used as relevant canvas fallback content.

Non-element focusable areas also exist: `area` element shapes, UA-provided subwidgets
(video controls, number spinners, the `details` twisty), **scrollable regions** of
rendered non-inert elements, and the **viewport** of a document. Each has a "DOM anchor"
— the node that stands in for it in APIs. So the web's focus target is *not* one-to-one
with the element tree; a scroll region is separately focusable from the element that
generates it.

> All elements that are focusable areas are said to be **focusable**.

**Sequentially focusable**:

> A focusable area is said to be sequentially focusable if it is included in its
> Document's sequential focus navigation order and the user agent determines that it is
> sequentially focusable.

**Click focusable**:

> A focusable area is said to be click focusable if the user agent determines that it is
> click focusable. User agents should consider focusable areas with non-null tabindex
> values to be click focusable.

**How they relate** — the spec's own note is the key paragraph for froth:

> Being focusable is a statement about whether an element can be focused
> programmatically, e.g. via the `focus()` method or `autofocus` attribute. In contrast,
> sequentially focusable and click focusable govern how the user agent responds to user
> interaction: respectively, to sequential focus navigation and as activation behavior.
>
> Elements which are not focusable are not focusable areas, and thus not sequentially
> focusable and not click focusable.
>
> Thus, an element can be focusable, but neither sequentially focusable nor click
> focusable. For example, in some user agents, a non-editable form-control with a
> negative-integer tabindex value would not be focusable via user interaction, only via
> programmatic APIs.

So the shape is **not three peers**. It is a base predicate (`focusable` = programmatic)
with two independent sub-flags layered on it, both strict subsets:

```
focusable  ⊇  sequentially focusable
focusable  ⊇  click focusable
(sequentially focusable and click focusable are independent of each other)
```

There is no way to be click focusable or sequentially focusable without being focusable.

**Both sub-flags are user-agent judgement, not author declaration.** The spec twice names
the escape hatches:

> The user agent might determine that an element is not sequentially focusable even if it
> is focusable and is included in its Document's sequential focus navigation order,
> according to user preferences. For example, macOS users can set the user agent to skip
> non-form control elements, or can skip links when doing sequential focus navigation with
> just the Tab key…
>
> Similarly, the user agent might determine that an element is not click focusable even if
> it is focusable. For example, in some user agents, clicking on a non-editable form
> control does not focus it…

And the click hook itself:

> When a user activates a click focusable focusable area, the user agent must run the
> focusing steps on the focusable area with focus trigger set to "click".
>
> Note that focusing is not an activation behavior, i.e. calling the `click()` method on
> an element or dispatching a synthetic click event on it won't cause the element to get
> focused.

That last sentence matters: **synthetic clicks do not focus.** Click-to-focus is a UA
input-handling behaviour, not a consequence of the click event.

### A.3 `tabindex`, precisely — and what froth's summary gets wrong

HTML §6.6.3 (<https://html.spec.whatwg.org/multipage/interaction.html#the-tabindex-attribute>).
Non-normative summary table, quoted:

> **omitted (or non-integer values)** — The user agent will decide whether the element is
> focusable, and if it is, whether it is sequentially focusable or click focusable (or
> both).
>
> **−1 (or other negative integer values)** — Causes the element to be focusable, and
> indicates that the author would prefer the element to be click focusable but not
> sequentially focusable. The user agent might ignore this preference for click and
> sequential focusability, e.g., for specific element types according to platform
> conventions, or for keyboard-only users.
>
> **0** — Causes the element to be focusable, and indicates that the author would prefer
> the element to be both click focusable and sequentially focusable. The user agent might
> ignore this preference for click and sequential focusability.
>
> **positive integer values** — Behaves the same as 0, but in addition creates a relative
> ordering within a tabindex-ordered focus navigation scope, so that elements with higher
> tabindex attribute value come later.

Normative processing model, negative case:

> The user agent **must** consider the element as a focusable area, but **should** omit the
> element from any tabindex-ordered focus navigation scope.
>
> One valid reason to ignore the requirement that sequential focus navigation not allow the
> author to lead to the element would be if the user's only mechanism for moving the focus
> is sequential focus navigation. For instance, a keyboard-only user would be unable to
> click on a text control with a negative tabindex, so that user's user agent would be well
> justified in allowing the user to tab to the control regardless.

Three corrections to the summary froth has in its notes:

1. **`tabindex="-1"` IS click focusable.** That is its stated intent, not an accident.
   It's the direct analogue of froth's question and the answer is yes: `-1` = focusable +
   clickable + not walkable. This is the single most load-bearing fact in this survey.
2. **The strength is asymmetric.** `must` be focusable; `should` be excluded from tab
   order; click focusability is only a *preference the UA might ignore*. Only "focusable"
   is a hard guarantee. Tabbability and clickability are hints.
3. **`tabindex` cannot subtract.** Spec note:
   > Note that the tabindex attribute cannot be used to make an element non-focusable. The
   > only way a page author can do that is by disabling the element, or making it inert.

   There is no `tabindex="none"`. Removing focusability is a *different mechanism* from
   ordering it (see §C).

**"tabindex focus flag" is a dead term.** It does not appear in the current HTML Standard;
`grep` over the live §6.6 text returns nothing. Issue #4607 explains why it was removed:

> It doesn't have an explicit concept of sequentially focusable. Instead it has a "tabindex
> focus flag" plus implicit UA heuristics… I think the tabindex focus flag is especially
> confusing…

The flag was replaced by the explicit `sequentially focusable` / `click focusable` pair.
If froth's notes cite it, they are citing pre-2019 vocabulary.

### A.4 Elements focusable with no `tabindex`

There is **no normative list**. HTML §6.6.3, under "If the [tabindex] value is null":

> The user agent should follow platform conventions to determine if the element should be
> considered as a focusable area…
>
> Modulo platform conventions, it is **suggested** that the following elements should be
> considered as focusable areas and be sequentially focusable:
>
> `a` elements that have an `href` attribute; `button` elements; `input` elements whose
> `type` attribute are not in the Hidden state; `select` elements; `textarea` elements;
> `summary` elements that are the first `summary` element child of a `details` element;
> elements with a `draggable` attribute set, if that would enable the user agent to allow
> the user to begin drag operations for those elements without the use of a pointing
> device; editing hosts; navigable containers.

"Suggested", not required. Ordering for null-tabindex elements "should be in
shadow-including tree order".

The one hard, normative surface is `HTMLElement.tabIndex`'s getter, which returns 0 for
`a`, `area`, `button`, `frame`, `iframe`, `input`, `object`, `select`, `textarea`, SVG `a`,
MathML `a`, and a `summary` that is a summary for its parent `details`; otherwise −1. That
is a reflection default, not a focusability statement, but it is the closest thing to a
canonical list.

### A.5 What happens when you click a non-focusable thing

**Per spec, nothing** — focus stays where it was. The relevant algorithms:

*Get the focusable area* for a focus target that is not itself a focusable area
(<https://html.spec.whatwg.org/multipage/interaction.html#get-the-focusable-area>) matches
the first of: an `area` with focusable shapes → its shape; an element with focusable
scrollable regions → its first scrollable region; the document element → the viewport; a
navigable → its active document; a navigable container → its content navigable's document;
a shadow host with `delegates focus` → its focus delegate. **Otherwise: return null.**

*Focusing steps*
(<https://html.spec.whatwg.org/multipage/interaction.html#focusing-steps>):

> 1. If new focus target is not a focusable area, then set new focus target to the result
>    of getting the focusable area for new focus target, given focus trigger…
> 2. If new focus target is null: If no fallback target was specified, then **return**.
>    Otherwise, set new focus target to the fallback target.

There is **no "nearest focusable ancestor" step anywhere**. The spec does not walk up.
Combined with §A.2 — the UA runs focusing steps only "when a user activates a *click
focusable* focusable area" — the spec's model is: click a plain `<span>`, and the focusing
steps are never invoked at all. Focus does not move.

**Common practice disagrees.** Browsers do move focus on mousedown over non-focusable
content — typically blurring to the body — and the ancestor walk that authors rely on
(clicking a `<span>` inside a `<button>` focuses the button) is unspecified. Issue #4607
was filed precisely because these "shaky spec foundations" exist, and the click-focus
input path is one of the parts still not written down. **UNVERIFIED**: I did not find a
normative spec passage describing the ancestor walk or the body fallback, and I did not
run browser tests. Treat "browsers walk to the nearest focusable ancestor / fall back to
`<body>`" as widely-observed behaviour with no spec backing, not as the platform's answer.

The only *specified* delegation is `delegatesFocus` on a shadow root (§C.5).

`document.activeElement` papers over the gap: if the focused area is a `Document`, the
getter returns that document's `body` element, else the document element, else null
(<https://html.spec.whatwg.org/multipage/interaction.html#dom-documentorshadowroot-activeelement>).
So "focus went to body" is partly an artifact of the API, not necessarily of the focus
state.

---

## B. Programmatic focus as a distinct axis

`HTMLElement.focus(options)` steps
(<https://html.spec.whatwg.org/multipage/interaction.html#dom-focus>):

> 1. If the allow focus steps given this's node document return false, then return.
> 2. Run the focusing steps for this.
> 3. If options["focusVisible"] is true, or does not exist but in an implementation-defined
>    way the user agent determines it would be best to do so, then indicate focus.
> 4. If options["preventScroll"] is false, then scroll a target into view given this,
>    "auto", "center", and "center".

`FocusOptions` is `{ boolean preventScroll = false; boolean focusVisible; }`.
`preventScroll` suppresses the scroll-into-view; `focusVisible` forces the focus ring on.
Neither has anything to do with whether focus is granted.

**Can `focus()` reach something outside the sequential order?** Yes — that is the entire
point of `tabindex="-1"`, and the note in §A.2 says so directly ("focusable via
programmatic APIs" only).

**Can `focus()` reach something with no `tabindex`?** Only if the UA already considers it
focusable (a `<button>`, an editing host, a scroll region…). On a plain `<div>` with no
`tabindex`, step 1 of the focusing steps gets null from *get the focusable area*, and the
algorithm returns. **`focus()` silently does nothing** — no throw, no signal. WHATWG issue
#2772 exists to ask for a console warning for exactly this
(<https://github.com/whatwg/html/issues/2772>), which is evidence it is a real and
frequently-hit footgun.

**Is `focus()` ever refused?** Three distinct refusals, all silent:

1. **Not a focusable area** — returns, above.
2. **Inert** — focusing steps: *"If new focus target is a focusable area and its DOM anchor
   is inert, then return."* Note that unfocusing has the mirror rule: *"If old focus target
   is inert, then return"* — you cannot blur an inert element either.
3. **Cross-document / permissions policy** — the *allow focus steps*:
   > 1. If target is allowed to use the "focus-without-user-activation" feature, then return
   >    true.
   > 2. If target's relevant global object has transient activation, then return true.
   > 3. Return false.

   A document with no transient activation and no policy grant cannot call `focus()` at
   all. This is anti-abuse machinery, not a focusability axis.

`blur()` is spec'd as "moves the focus to the viewport", and the spec explicitly permits
UAs to ignore it: *"User agents may instead selectively or uniformly do nothing, for
usability reasons."*

**Verdict on B**: the web genuinely has three axes, but not three symmetric ones. It has
one axis that is a real capability check (`focusable`, the only script-observable one) and
two that are *routing policies over user input* which the UA may override on user
preference. froth's "focusable by name" corresponds to the base capability; "walkable" and
"clickable" correspond to the two policies.

---

## C. Hit-test transparency

### C.0 The uncomfortable baseline: hit testing is not specified

CSS UI Level 4 §6.2 (<https://drafts.csswg.org/css-ui/#pointer-events-control>), in the
spec that defines `pointer-events`:

> While this property modifies the normal behavior of hit-testing, this normal hit-testing
> is currently not specified. There is broad interoperability about the seemingly obvious
> parts of this problem, but there are countless nuances and corner cases that would greatly
> benefit from a detailed specification. The CSS-WG would hugely appreciate help with
> writing such a specification.

CSSOM View concurs
(<https://drafts.csswg.org/cssom-view/#dom-document-elementfrompoint>):

> Note: The specifics of hit testing are out of scope of this specification and therefore
> the exact details of `elementFromPoint()` and `caretPositionFromPoint()` are therefore
> too. Hit testing will hopefully be defined in a future revision of CSS or HTML.

So froth is copying a part of the platform that the platform never wrote down. Everything
below is real spec text about *modifications* to an unspecified base.

### C.1 `pointer-events`

CSS UI 4 §6.2. `auto | none`, initial `auto`, **inherited: yes**, applies to all elements.

> **auto** — Hit-testing occurs normally.
>
> **none** — Hit-testing acts as if the boxes generated by the element (see [CSS-DISPLAY-3])
> were not there, effectively causing the element behind the `pointer-events: none` element
> to become the target of hit-testing-based events instead.

**Beneath in paint order, not the parent.** The element "behind" it. The box is removed
from consideration and the search continues at that point; it does not redirect to an
ancestor. Confirmed by CSSOM View's note on `elementFromPoint`: *"The elementFromPoint()
method does not necessarily return the top-most painted element. For instance, an element
can be excluded from being a target for hit testing by using the pointer-events CSS
property."*

Three qualifications the spec states:

> `pointer-events: none` does not inhibit sequential focus navigation.

> Note: This `pointer-events: none` only excludes the element itself from hit-testing, not
> the whole subtree. While this property propagates to descendants via inheritance,
> `pointer-events: auto` can be specified on a descendant of a `pointer-events: none`
> element, and such an element will participate normally in hit-testing.

> Note: Events that target a `pointer-events: auto` descendant of a `pointer-events: none`
> element can still be caught by event handlers on `pointer-events: none` ancestors of the
> targeted element during the bubbling or capturing phases.

That is the mechanism froth would want: **descendant-transparency is achieved by
inheritance, not by subtree semantics**, so a child can re-opt-in with a single
declaration, and the transparent ancestor still sees the event on the way up.

Also flagged as unresolved by the CSS-WG:

> The statement above needs to be qualified; for some purposes, hit-testing continues to
> take the element into account. For instance, clicking and dragging to start a text
> selection will continue to work as usual. What is the precise set of things for which
> hit-testing is affected by pointer-events?

So `pointer-events: none` is not even a clean "invisible to pointers" in its own spec.

SVG defines a variant with more values (`visiblePainted`, `stroke`, `fill`, …); CSS UI says
"the effect of such values outside of SVG is currently not defined."

### C.2 `inert`

HTML §6.3 (<https://html.spec.whatwg.org/multipage/interaction.html#inert-subtrees>).
Inertness is a **node** property, not just an element attribute:

> A node (in particular elements and text nodes) can be inert. When a node is inert:
>
> - Hit-testing must act as if the 'pointer-events' CSS property were set to 'none'.
> - Text selection functionality must act as if the 'user-select' CSS property were set to
>   'none'.
> - If it is editable, the node behaves as if it were non-editable.
> - The user agent should ignore the node for the purposes of find-in-page.
>
> Inert nodes generally cannot be focused, and user agents do not expose the inert nodes to
> accessibility APIs or assistive technologies. Inert nodes that are commands will become
> inoperable to users…

So **`inert` is defined in terms of `pointer-events: none` plus four more things.** It is
the strictly larger hammer: unfocusable (per the focusable-area criteria, §A.2), unhittable,
unselectable, uneditable, unsearchable, invisible to AT.

The `inert` attribute:

> The `inert` attribute is a boolean attribute that indicates, by its presence, that the
> element and all its **flat tree descendants** which don't otherwise escape inertness (such
> as modal dialogs) are to be made inert by the user agent.

**Descendants: yes, via the flat tree, and it is escapable.** Unlike `pointer-events`, this
is not CSS inheritance — it is subtree semantics with a defined escape.

**Yes, `<dialog>` modality is defined in terms of inert.** HTML §6.3.1:

> A Document `document` is blocked by a modal dialog `subject` if `subject` is the topmost
> `dialog` element in `document`'s top layer. While `document` is so blocked, every node
> that is connected to `document`, with the exception of the `subject` element and its flat
> tree descendants, **must become inert**.
>
> `subject` can additionally become inert via the `inert` attribute, but only if specified
> on `subject` itself (i.e., `subject` escapes inertness of ancestors); `subject`'s flat
> tree descendants can become inert in a similar fashion.
>
> The `dialog` element's `showModal()` method causes this mechanism to trigger, by adding
> the `dialog` element to its node document's top layer.

That is the wiring: modality is *not* a focus trap. It is "make the entire document inert
except this subtree", and the focus consequences fall out of the focusable-area criteria.
The escape rule — a subtree can re-enable itself out of an inert ancestor — is what makes
this composable.

Support (per the spec's own compat data): all current engines — Firefox 112+, Safari 15.5+,
Chrome 102+, Edge 102+.

**CSS equivalent, shipping**: CSS UI 4 §6.3 defines the `interactivity: auto | inert`
property (inherited, all elements) with the identical four-bullet list, *"Hit-testing must
act as if `pointer-events` was `none`, regardless of its actual value."* Note from the spec:
*"This property allows an author to force an element inert, but doesn't allow making an
element *non*-inert"* — the CSS form has no escape hatch, unlike the attribute. Shipped in
Chrome/Edge/WebView 135 (desktop + Android) as "CSS Inertness"
(<https://chromestatus.com/feature/5107436062146560> — feature name "CSS Inertness",
`interactivity` property, enabled by default since 135). Firefox and WebKit: **no signal**
as of today (mozilla/standards-positions#1161, WebKit/standards-positions#447). So this one
is Chromium-only.

### C.3 `disabled` — a third, different mechanism

HTML §4.15 (<https://html.spec.whatwg.org/multipage/semantics-other.html#concept-element-disabled>):

> An element is said to be **actually disabled** if it is one of the following: a `button`
> element that is disabled; an `input` element that is disabled; a `select` element that is
> disabled; a `textarea` element that is disabled; an `optgroup` element that has a
> `disabled` attribute or whose nearest ancestor `select` is disabled; an `option` element
> that is disabled or whose nearest ancestor `select` is disabled; a `fieldset` element that
> is a disabled fieldset; a form-associated custom element that is disabled.
>
> This definition is used to determine what elements are focusable and which elements match
> the `:enabled` and `:disabled` pseudo classes.

Clicks — HTML §4.10.19.5
(<https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#attr-fe-disabled>):

> A form control that is disabled **must prevent any `click` events that are queued on the
> user interaction task source from being dispatched on the element**.

This is a *narrower and differently-shaped* mechanism than `inert`:

| | `disabled` | `inert` | `pointer-events: none` |
|---|---|---|---|
| applies to | only the listed form controls | any node | any element |
| removes focusability | yes (via "actually disabled" in the focusable-area criteria) | yes (via "not inert" in the same criteria) | **no** — and explicitly does not inhibit tabbing |
| hit testing | element is still hit; `click` is suppressed at dispatch | acts as `pointer-events: none` | element's boxes skipped |
| other events (mouseover, pointerdown) | still dispatched | suppressed with the hit test | suppressed with the hit test |
| descendants | via `fieldset`/`select`/`optgroup` containment rules only | flat-tree subtree, escapable | CSS inheritance, re-openable per element |
| hidden from AT | no (exposed as disabled) | yes | no |

The crucial contrast for froth: **`disabled` suppresses the *result* of a hit
(`click` dispatch) while `pointer-events: none` suppresses the *hit itself*.** Those are
different layers, and the platform needed both. Note also that `disabled` does not stop
everything: *"Being disabled does not prevent all modifications to the form control. For
example, the control's value or checkedness could be modified programmatically…"*

### C.4 `visibility: hidden` / `display: none` / `opacity: 0`

| | generates boxes? | "being rendered" (HTML)? | focusable? | hit-tested? |
|---|---|---|---|---|
| `display: none` | no | **no** | no | no |
| `visibility: hidden` | **yes** | **yes** | no in practice | no in practice |
| `opacity: 0` | yes | yes | yes | **yes** |

`display: none` — HTML defines *being rendered* as
(<https://html.spec.whatwg.org/multipage/rendering.html#being-rendered>):

> An element is being rendered if it has any associated CSS layout boxes, SVG layout boxes,
> or some equivalent in other styling languages.
>
> Just being off-screen does not mean the element is not being rendered.

`display: none` generates no boxes → not being rendered → fails the focusable-area
criteria → not focusable. This is the one clean, spec-derivable case.

`visibility: hidden` — CSS 2.2 §11.2
(<https://www.w3.org/TR/CSS22/visufx.html#visibility>):

> **hidden** — The generated box is invisible (fully transparent, nothing is drawn), but
> still affects layout. Furthermore, descendants of the element will be visible if they have
> 'visibility: visible'.

Boxes **are** generated, so per the HTML criteria a `visibility: hidden` element *is*
"being rendered" and would qualify as a focusable area. CSS UI 4 confirms the box
generation: *"An element might be invisible but still generate boxes. For example, if the
element has `visibility: hidden` but not `display: none`, boxes are generated for it…"*.
**UNVERIFIED**: I found no normative text in HTML, CSS 2.2, CSS UI, or CSSOM View stating
that `visibility: hidden` removes an element from hit testing or from focus. Browsers
uniformly do both; the spec basis is the unwritten hit-testing algorithm (§C.0). Report
this as interoperable-but-unspecified.

`opacity: 0` — nothing in any of these specs excludes it from anything. It is a fully
hittable, fully focusable, fully tabbable invisible element. This is the standard
accessibility trap (the invisible tab stop) and the standard trick (the invisible
click-target overlay).

**This is froth's cleanest lesson from the web**: visibility and hittability are entirely
orthogonal on the platform, and the platform has one of each corner in daily use —
`opacity: 0` (visible-to-hit, invisible-to-eye) and a `pointer-events: none` overlay
(visible-to-eye, invisible-to-hit).

### C.5 "Skip me and continue beneath" vs "hit me but hand the result elsewhere"

These are genuinely different mechanisms on the web and the platform names both.

**Skip and continue beneath** — `pointer-events: none`, `inert`, `interactivity: inert`.
The box is removed from the hit-test search; the element behind becomes the target
(§C.1). The result goes *down the z-stack*, not *up the tree*.

**Hit me, redirect the result** — three named mechanisms, all redirecting to a *specific*
other element, never generically "my parent":

1. **`delegatesFocus`** — HTML *get the focusable area*: if the focus target is a shadow
   host whose shadow root's `delegates focus` is true, return its **focus delegate** — the
   autofocus delegate if any, otherwise the first descendant in tree order that is a
   focusable area (with the recursion through nested hosts). The host is hit; the focus
   lands inside. Notably the delegate search is filtered by trigger: *"If focusable area is
   not click focusable and focus trigger is 'click', then continue."* — click focusability
   is consulted when choosing the delegate.
   <https://html.spec.whatwg.org/multipage/interaction.html#focus-delegate>
2. **`<label>`** — HTML §4.10.4: *"The `label` element's exact default presentation and
   behavior, in particular what its activation behavior might be, if anything, should match
   the platform's label behavior."* On platforms where it applies, clicking the label
   activates and focuses the labeled control. And, importantly, the redirect is suppressed
   for interactive descendants: *"The activation behavior of a `label` element for events
   targeted at interactive content descendants of a `label` element, and any descendants of
   those interactive content descendants, must be to do nothing."*
   <https://html.spec.whatwg.org/multipage/forms.html#the-label-element>
3. **`area` shapes and scroll regions** — *get the focusable area* substitutes the
   corresponding non-element focusable area for the element that was targeted.

**Event bubbling is the fourth, and it is not a focus mechanism.** CSS UI's note (§C.1) —
handlers on a `pointer-events: none` ancestor still see events targeted at an `auto`
descendant during capture/bubble. So on the web, "my parent gets to know about the click"
is a property of the *event dispatch tree*, entirely separate from "my parent gets focus",
which is a property of the *focus algorithms*. The platform never conflates them.

**There is no generic parent-delegation.** The ancestor walk authors believe in (click the
`<span>`, focus the `<button>`) is the unspecified UA behaviour from §A.5, not a mechanism
with a name.

---

## D. Composite widgets: roving tabindex and `focusgroup`

### D.1 The APG convention

APG Keyboard Interface practice
(<https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/>):

> As described in section Fundamental Keyboard Navigation Conventions, **the tab sequence
> should include only one focusable element of a composite UI component.** Once a composite
> contains focus, keys other than Tab and Shift + Tab enable the user to move focus among
> its focusable elements.
>
> Authors are free to choose which keys move focus inside of a composite, but they are
> strongly advised to use the same key bindings as similar components in common GUI
> operating systems…

This is exactly froth's "one thing you are inside of". Tab operates at the composite level;
arrows operate at the member level. **Two levels of hierarchy, two key bindings** — the
platform's answer to "tab and arrows operate at different levels" is that the *component*,
not the platform, decides which arrows do what, and the tab sequence is collapsed to one
entry.

**Where focus lands on re-entry — three different conventions, per widget type**, and the
APG is explicit that this is a design decision, not a default:

> - The element that had focus the last time the composite contained focus. Or, if the
>   composite has not yet contained the focus, the first element. Widgets that usually
>   employ this pattern include **grid and tree grid**.
> - The selected element. Or, if there is no selected element, the first element. Widgets
>   where this pattern is commonly implemented include **radio groups, tabs, list boxes, and
>   trees**.
> - The first element. Components that typically follow this pattern include **menubars and
>   toolbars**.

So "remember where I was" is only one of three, and the APG assigns it to grid/treegrid —
not to lists or tabs, where *selection* wins over *recency*.

### D.2 Roving tabindex — the algorithm, verbatim

> When using roving tabindex to manage focus in a composite UI component, the element that
> is to be included in the tab sequence has `tabindex="0"` and all other focusable elements
> contained in the composite have `tabindex="-1"`. The algorithm for the roving tabindex
> strategy is as follows.
>
> 1. When the component container is loaded or created, set `tabindex="0"` on the element
>    that will initially be included in the tab sequence and set `tabindex="-1"` on all
>    other focusable elements it contains.
> 2. When the component contains focus and the user presses a navigation key that moves
>    focus within the component, such as an arrow key:
>    - set `tabindex="-1"` on the element that has `tabindex="0"`.
>    - Set `tabindex="0"` on the element that will become focused as a result of the key
>      event.
>    - Set focus, `element.focus()`, on the element that has `tabindex="0"`.
> 3. If the design calls for a specific element to be focused the next time the user moves
>    focus into the composite with Tab or Shift + Tab, check if that target element has
>    `tabindex="0"` when the composite loses focus. If it does not, set `tabindex="0"` on
>    the target element and set `tabindex="-1"` on the element that previously had
>    `tabindex="0"`.

Note what the container does: **nothing**. The container is not focusable and holds no
state. All of the mechanism lives in the members' `tabindex` values.

Note also the pattern's dependency: `tabindex="-1"` members are still click-focusable
(§A.3), so a mouse user can land anywhere in the composite and the widget then has to
repair its own invariant.

### D.3 Is the memory state, or derivable?

**It is state, and it lives in the DOM.** The element currently carrying `tabindex="0"`
*is* the memory. There is no separate register. Two consequences:

- **The memory is automatic in the "last-focused" convention and manual otherwise.** Step 2
  updates `tabindex="0"` on every arrow-key move, so recency-memory is a free side effect of
  the algorithm. Step 3 exists precisely because the other two conventions (selected
  element, first element) require the author to *overwrite* the memory on blur.
- The Open UI explainer says this in as many words:
  > This behavior is akin to the roving tabindex in which the "memory" is the **stateful
  > `tabindex="0"` value** assigned to the currently focused element.

  <https://open-ui.org/components/scoped-focusgroup.explainer/>

So: derivable from DOM inspection, but only because the DOM is being mutated to carry it.
The web chose to store the memory *in the tab-order axis itself* rather than in a
container-level field. `focusgroup` (D.5) reverses that choice and makes it UA-internal
state with defined invalidation.

### D.4 `aria-activedescendant` — the other strategy

APG, same page:

> If a component container has an ARIA role that supports the `aria-activedescendant`
> property, it is not necessary to manipulate the `tabindex` attribute and move DOM focus
> among focusable elements within the container.
>
> When the container has DOM focus, the value of `aria-activedescendant` on the container
> tells assistive technologies which element is active within the widget. Assistive
> technologies will consider the element referred to as active to be the focused element
> even though DOM focus is on the element that has the `aria-activedescendant` property.
> And, when the value of `aria-activedescendant` is changed, assistive technologies will
> receive focus change events equivalent to those received when DOM focus actually moves.

The difference is precise and structural:

| | roving tabindex | `aria-activedescendant` |
|---|---|---|
| where real focus is | on the **member** | on the **container** |
| what the members are | each focusable, one tabbable | need not be focusable at all |
| where "active" is recorded | member's `tabindex` (DOM) | container's IDREF attribute |
| scroll-into-view | free, from the UA | author's job |

The APG names that last one as the deciding tradeoff:

> One benefit of using roving tabindex rather than `aria-activedescendant` to manage focus is
> that the user agent will scroll the newly focused element into view.

And it warns that `aria-activedescendant` constrains the DOM relationship between the
focused container and the active element (the container must be the active element's
ancestor, or a combobox/textbox/searchbox with `aria-controls` to it).

**This is a genuinely different answer to froth's container problem**: rather than making
the container "one thing you are inside of" by collapsing its members' tab stops, make the
container the *only* focus target and track the active member as a separate, explicit,
container-level field. Two members-focusable and container-focusable models, both in
production.

### D.5 `focusgroup` — status as of 2026-08-27, verified live

**Status (verified from primary sources today, not from training data):**

- **Chromium: shipped.** ChromeStatus feature 5637601087193088, "Focusgroup" — origin trial
  desktop+Android 146–149, **shipped desktop + Android in 150**. Chrome 150 reached stable
  ~2026-07-14 (chromiumdash milestone schedule). Summary text, verbatim: *"Gives authors the
  ability to declaratively give composite widgets arrow key navigation, a guaranteed tab
  stop, and last-focused memory, replacing hand-coded roving tabindex scripts."*
  <https://chromestatus.com/feature/5637601087193088>
- **WHATWG HTML: PR open, not merged.** whatwg/html#11723, state OPEN, title "[focusgroup]
  Add the focusgroup attribute", against issue #11641. Its own checklist records: Chromium
  shipped in 150; Mozilla standards position **positive** (mozilla/standards-positions#1348);
  WebKit position **unanswered** (WebKit/standards-positions#171); WPT tests exist under
  `html/interaction/focus/focusgroup/tentative`, marked tentative pending consensus.
  <https://github.com/whatwg/html/pull/11723>
- **Open UI: active proposal**, explainer last updated 2026-07-28, at
  <https://open-ui.org/components/scoped-focusgroup.explainer/> (the older
  `/components/focusgroup.explainer/` URL is now 404 — the scoped explainer superseded it).
- **Firefox / Safari: not shipping.** Positive signal from Mozilla, no signal from WebKit.
- A follow-on, **"FocusgroupV2: grid and feed"** (grid two-dimensional navigation, `feed`,
  an `itemcontrols` modifier making nested controls sequentially reachable only for the
  active item), is at ChromeStatus 5168133036965888, intent stage 1 — proposed only, not
  shipped. So one browser, one unmerged PR, and a v2 already in flight.

**What it specifies.** Syntax: `focusgroup="<behavior> [inline|block] [wrap|nowrap] [nomemory]"`,
tokens space-separated, any order. Author-facing summary from the explainer:

> Authors may use the proposed `focusgroup` HTML attribute to declare that a subtree of
> focusable elements will get:
> - Focus navigation (not selection) using directional navigation (arrow key, D-pads, etc.).
> - **A guaranteed tab stop** (when at least one focusable element is present).
> - **Automatic return to the last focused focusable element** (unless `nomemory` is set).
> - Optional limited-axis arrow key navigation and optional wrap-around semantics for linear
>   behaviors.

Six behavior tokens, each carrying default modifiers and a minimum ARIA role:

| behavior | APG pattern | min container role | min child role | default modifiers |
|---|---|---|---|---|
| `toolbar` | Toolbar | toolbar | (none) | `inline` |
| `tablist` | Tabs | tablist | tab | `inline wrap` |
| `radiogroup` | Radio Group | radiogroup | radio | `wrap` |
| `listbox` | Listbox | listbox | option | `block` |
| `menu` | Menu | menu | menuitem | `block wrap` |
| `menubar` | Menubar | menubar | menuitem | `inline wrap` |

Modifier tokens: `inline`/`block` restrict the arrow axis (omit for both); `wrap`/`nowrap`
control end→start looping; `nomemory` disables memory; `none` is a standalone opt-out that
cannot combine with anything. Companion attribute `focusgroupstart` marks the item focused
on first entry.

**Guaranteed tab stop:**

> Focusgroups ensure that **exactly one focusgroup item serves as the sequential entry point
> for each segment**, unless there are no focusable items… Note that even if multiple
> descendants are naturally tabbable (e.g., several `<button>`s without `tabindex`),
> focusgroup still collapses the group to a single sequential entry point per segment.
> **Authors do NOT need to manually set `tabindex="-1"` on every non-active item** to gain
> roving behavior.

Entry precedence, from the worked example: (1) an element inside with an explicit
`tabindex >= 0`; (2) the memory; (3) the first item in DOM order (or reading order under
`reading-flow`).

**Memory — now UA state with defined invalidation:**

> By default, focusgroups will remember the last-focused element, and for sequential focus
> navigation, will restore focus to that element when a focusgroup is re-entered, **if it is
> present in the focusgroup segment being entered**… The focusgroup's memory is initially
> empty. In that state, sequential focus navigation will pick the next element to focus using
> existing platform behavior…

The memory **must** be cleared when: the focusgroup element or the remembered element is
hidden/un-hidden; either has its `disabled` or `inert` status changed; either is removed from
the shadow-inclusive tree; the remembered element stops being focusable (e.g. its `tabindex`
is removed); or the remembered element becomes excluded from the focusgroup. That list is
the interesting part for froth — it is the complete set of ways a remembered handle can go
stale, written out by people who had to implement it.

`nomemory` exists for the case where recency is the wrong answer:

> Usually this is because there is a more relevant element which should take focus when
> entering the focusgroup instead of the most-recently-focused element. For example, an
> active (selected) tab in a `role="tablist"` container, rather than the last-focused tab
> (when selection doesn't follow focus).

That is the APG's "selected element" convention (§D.1), reappearing as a flag.

**Segments — the part froth should read twice.** A focusgroup is not necessarily one tab
stop; it is one tab stop *per segment*:

> A focusgroup segment is a contiguous run of focusgroup items that shares one entry point in
> sequential focus navigation. The following excluded content creates a boundary when it
> contains a sequentially focusable area or the currently focused area: an element opting out
> through `focusgroup="none"` and its excluded descendants; an independent nested focusgroup
> with focusgroup items; a top-layer excluded subtree.
>
> Directional navigation may cross segments while skipping the excluded content; **segments
> constrain sequential entry rather than directional reachability.**
>
> Non-opted-out focusgroup items with `tabindex="-1"` do not create segment boundaries…
> When focused by other means (e.g., programmatically or by pointer), they participate in
> directional navigation within their containing segment but do not split it.

Worked example: items A B C D E with C opted out and sequentially focusable → segments
[A, B] and [D, E]. Arrows cross freely; Tab sees two stops. And note the last sentence:
**pointer and programmatic focus can land on a member the tab sequence cannot reach, and
that member then participates in arrow navigation.** All three of froth's axes are live
simultaneously in that sentence.

**Nesting:**

> When a focusgroup definition is applied to an element, it implicitly opts out of any
> ancestor's focusgroups. This ensures that **every element can only belong in one focusgroup
> at a time.**

Nesting is therefore *not* recursive containment — a nested focusgroup is a sibling in
behaviour, reached by Tab, not by arrows. The explainer's menubar → menu → submenu example
makes each level an independent scope.

Top-layer elements (shown popovers, modal dialogs, fullscreen) are excluded from ancestor
focusgroups while in the top layer, with the same segment-boundary effect as
`focusgroup="none"`, and rejoin when they leave. The stated reason is spatial: *"Keeping such
elements in the ancestor's arrow key traversal would let focus jump to content rendered at a
completely different location on screen."*

**Key conflicts** — the honest failure mode. A member that itself consumes arrow keys
(`<input>`, `<textarea>`, `<select>` on the same axis, `contenteditable`, focusable scroll
regions on the same axis, custom elements with arrow handlers, anything calling
`preventDefault()`) wins:

> When there is a conflict between the arrow keys consumed by the interactive element and the
> focusgroup's navigation, focusgroup will not interfere with the interactive element's
> behavior — the normal way to move focus between focusgroup items (arrow keys) won't work
> when focus is within such an element.

The UA-provided escape for native conflicts is Tab: *"pressing Tab moves focus to the next
focusable element in the focusgroup (if any), and Shift+Tab moves focus to the previous
one."* — i.e. inside a conflicting member, Tab temporarily demotes to member-level
navigation. Conflict is detected per-axis: a vertical scroll container inside an `inline`
focusgroup is not a conflict. For scripted handlers the explainer offers no mechanism, only
advice (limit the axis, opt out with `focusgroup="none"`, or require an explicit
activation/Escape step to enter and leave).

### D.6 Tab and arrows at different levels — the platform's full inventory

- **APG convention (D.1)** — Tab enters/leaves the composite, arrows move within. Author-
  implemented, zero platform support, universal in practice.
- **`focusgroup` (D.5)** — the same split, declared, with UA-owned memory. One engine.
- **`aria-activedescendant` (D.4)** — Tab moves DOM focus to the container; arrows move a
  container-level pointer. Focus never enters the members.
- **Nested focusgroups (D.5)** — deliberately *not* a deeper level: nesting resets to a
  sibling scope, so the platform stops at two levels. There is no arrow-into-arrow
  hierarchy. The nearest thing is the unshipped FocusgroupV2 `itemcontrols` modifier, which
  makes nested controls sequentially reachable only for the active item — **UNVERIFIED**
  beyond its ChromeStatus summary; it is at intent stage 1 with no spec text I could find.
- **Modal dialog (`inert`, C.2)** — a level of a different kind: not a navigation level but a
  reachability level, implemented by making everything else unfocusable rather than by
  scoping the walk.

---

## Verdict: what transfers

**Transfers directly, and is well-tested:**

1. **The trichotomy is real and the web arrived at it the hard way.** WHATWG issue #4607
   enumerated exactly froth's three axes and the spec was rewritten to name two of them
   (`sequentially focusable`, `click focusable`) on top of a base (`focusable`). froth's
   hypothesis is not speculative; it is a platform that already ran the experiment.
2. **`tabindex="-1"` = focusable + clickable + not walkable.** The platform's central,
   most-used composite-widget primitive occupies precisely the cell froth is asking whether
   it needs. It needs it.
3. **The axes are not peers.** `focusable` is a capability; the other two are *routing
   policies over an input channel* and both are UA-overridable on user preference. Whether
   froth wants a "surface" flag or a "how does this input source route" policy is the shape
   question the web's structure poses.
4. **Removing focusability is a different mechanism from ordering it.** `tabindex` cannot
   subtract; only `disabled` and `inert` can. The platform kept "where in the walk" and
   "in the walk at all" in separate vocabulary and it has stayed clean for years.
5. **Visibility and hittability are orthogonal, with live examples in both off-diagonal
   corners** (`opacity: 0` = hittable and invisible; `pointer-events: none` overlay =
   visible and unhittable). This is worth copying as a stated invariant rather than
   discovered later.
6. **"Skip me" and "redirect me" are different mechanisms and the platform never merged
   them.** `pointer-events: none` continues the search *beneath in paint order*;
   `delegatesFocus` and `<label>` redirect to a *named specific* target. There is no
   generic "give it to my parent" — the closest thing (the ancestor walk) is the
   unspecified part that causes interop bugs.
7. **The container problem has two production answers, not one** (roving tabindex vs
   `aria-activedescendant`), differing in whether real focus enters the members. Which one
   fits depends on whether the members need to be independently focusable at all.
8. **`focusgroup`'s memory-invalidation list** (D.5) is free implementation experience: the
   five conditions under which a remembered member handle must be dropped, written by people
   shipping it.
9. **Re-entry has three conventions, not one.** Last-focused is the grid/treegrid answer;
   *selected* is the tabs/listbox/radiogroup answer; *first* is the toolbar/menubar answer.
   A design that hardcodes "return to where you left" will be wrong for tabs.

**Transfers with the shape distorted:**

- **`inert` is defined as "`pointer-events: none` + five more things."** The layering is the
  lesson, but the base layer is a CSS property froth does not have. froth would have to
  invent the base before it could define the composite.
- **`inert`'s escapability** (a modal dialog escapes its ancestors' inertness) is what makes
  modality composable — and it is defined over the *flat tree*, which is a DOM concept.
  froth's surface tree may or may not be able to express "everything except this subtree".
- **`focusgroup` segments** — the idea that a container can be *more than one* tab stop when
  something un-walkable interrupts it is a strong, non-obvious finding. But the boundary
  condition is defined in terms of DOM order and top-layer membership. froth has z-order and
  a surface tree, so the analogue exists but the definition would have to be rebuilt.

**Does not transfer — the web's answer is load-bearing on things froth lacks:**

- **Paint order.** "The element behind the `pointer-events: none` element becomes the target"
  presumes a z-ordered stack of overlapping boxes and a point query against it. froth has
  surfaces with a stacking order, so this may transfer — but the web's whole hit-test model
  is a *continue-searching-downward* loop, and CSS UI admits the loop itself is unspecified.
  froth would be copying a hole.
- **Inheritance as the descendant mechanism.** `pointer-events` gets subtree behaviour for
  free from CSS inheritance, and gets per-child re-enabling for free from the same
  mechanism. Without a cascade, froth has to choose subtree-vs-self explicitly and design
  the escape hatch by hand — which is exactly what `inert` (flat-tree subtree + escape rule)
  had to do, and `inert` is visibly more complicated than `pointer-events` as a result.
- **Event bubbling with capture.** The web can say "the transparent ancestor still sees the
  event" because dispatch and focus are separate trees. Any froth mechanism that wants
  "parent is notified but child is not the target" is inventing a dispatch model, not a
  focus flag.
- **The accessibility tree.** Half of what `inert` does (hide from AT) and all of
  `aria-activedescendant` are about a second, parallel representation for assistive tech.
  froth has one representation. `aria-activedescendant` in particular is *only* coherent
  because AT can be told "the active thing is over there" while DOM focus stays on the
  container — froth has no such channel, so its analogue would have to be a real focus
  concept.
- **UA/user override.** Both of the web's interaction axes are `should`, not `must`,
  precisely so macOS's "skip links when tabbing" setting is conforming. froth is the user
  agent *and* the toolkit, so it does not get to punt the hard cases to a preference the way
  the spec does. Every "the user agent might determine…" in §A is a decision froth must
  actually make.
- **Hit testing is not specified at all.** The single most-cited thing froth wants to copy
  (`pointer-events: none`) modifies an algorithm neither CSS nor HTML has ever written down,
  and both specs say so in a note. There is no normative prior art here to follow — only
  interoperable behaviour and two open pleas for someone to spec it.

**Recency check on the moving parts:** `focusgroup` ships in Chromium 150 only (stable
~2026-07-14) with an open WHATWG PR, Mozilla positive, WebKit silent — do not treat it as a
platform commitment. `interactivity: inert` ships in Chromium 135 only, with no signal from
either other engine. The `inert` *attribute* is the one in this cluster that is genuinely
universal (all current engines since Firefox 112 / Safari 15.5 / Chrome 102).
