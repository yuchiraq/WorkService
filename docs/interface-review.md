# Interface Review

The existing muted brand palette, compact lists, mobile timesheet matrix and
functional behavior are retained. This is an interaction and layout improvement,
not a switch to an Apple or Material visual theme.

## Guidelines Applied

- [Apple: Layout](https://developer.apple.com/design/human-interface-guidelines/layout):
  continuous background beneath navigation, full-width desktop workspace, related
  fields kept together, safe-area spacing and layouts that accommodate larger text.
- [Apple: Accessibility](https://developer.apple.com/design/human-interface-guidelines/accessibility):
  keyboard focus, descriptive control names, reduced motion, readable contrast and
  persistent feedback instead of time-limited errors.
- [Apple: Modality](https://developer.apple.com/design/human-interface-guidelines/modality):
  named, dismissible forms; focus contained in the active layer; inaccessible
  background controls; focus returned to the opening control.
- [Apple: Feedback](https://developer.apple.com/design/human-interface-guidelines/feedback):
  saving state, success confirmation, actionable errors in context and retained
  form values after network or validation failures.
- [Material: States](https://m3.material.io/foundations/interaction/states/overview):
  consistent hover, focus, pressed, disabled and pending states without moving
  surrounding controls.
- [Material: Text fields](https://m3.material.io/components/text-fields/guidelines):
  persistent labels, nearby validation messages, appropriate input keyboards and
  48 CSS-pixel mobile controls. Desktop forms use columns rather than a single
  excessively long input across the display.
- [Material: Layout examples](https://m3.material.io/foundations/layout/canonical-examples/overview):
  primary/supporting sections on wide displays; stacked sections on small displays.

## Implementation

- `web/static/css/style.css`: layout, appearance, responsive type, focus states.
- `web/static/js/ui.js`: validation, form submission feedback, focus containment,
  inactive layers, keyboard-accessible tables and measured header offset.
- `internal/api/shared.go`: navigation semantics and modal lifecycle integration.
- Worker/object confirmation dialogs use the browser's native dialog primitive.
- CSS, JavaScript and service-worker asset versions must be updated together.

POST forms are progressively enhanced; server authorization, CSRF and validation
remain authoritative. A failed request is never retried automatically. An
uncertain network result asks the person to check whether their record exists.
Form values are retained in the current page, not stored on disk or across reloads.

## Verification

Use the isolated fixture server, not production data:

```powershell
$env:WORKSERVICE_UI_PREVIEW='1'
go test ./internal/router -run TestSiteIntegration -v -timeout 0
```

In another terminal, with Node.js, Playwright and Microsoft Edge available:

```powershell
node scripts/ui-ux-check.cjs
```

`UI_BASE_URL` defaults to `http://127.0.0.1:8100`. `UI_OUTPUT_DIR` selects the
report/screenshot directory. `AXE_PATH` optionally points to a local axe-core
script for automated WCAG A/AA checks.

The suite covers 22 pages in light/dark appearance at 320, 390, 768, 1024, 1440,
1920 and 2560 pixels (308 combinations), full-width layout, continuous background,
control labels, keyboard navigation, validation, duplicate submission protection,
network errors, modal focus restoration and 200% text size. Screenshots complement
the automated assertions. Automated accessibility results do not establish full
WCAG conformance or replace testing with assistive-technology users and real iOS
and Android devices.
