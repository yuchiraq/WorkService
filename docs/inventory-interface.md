# Inventory interaction decisions

The inventory uses the existing site theme, not a copy of native iOS or Android styling.

- [Apple HIG: Search fields](https://developer.apple.com/design/human-interface-guidelines/search-fields): inline search above its collection, immediate results, clear action and explicit scope. Search matches words in any order; material and availability filters start with all items.
- [Material 3: Lists](https://m3.material.io/components/lists/overview): predictable ordering and consistent actions. Material groups and natural alphabetical sorting remain; desktop rows keep quantities beside names. On narrow screens, names use the full row above the controls.
- [Apple UI design tips](https://developer.apple.com/design/tips/) and [Material Web touch targets](https://github.com/material-components/material-components-web/blob/master/packages/mdc-touch-target/README.md): no horizontal scrolling, related controls close together, generous hit areas. Quantity inputs and icon controls have 48px targets; numeric fields retain 16px text.
- Search stays available when scrolling a long list, below the existing navigation. Sticky behavior is disabled in short landscape windows. Secondary actions remain in the row menu; the menu opens upward near the bottom edge.
- Material filters also prefill new items. Unsaved quantity drafts survive filtering and refreshes. Version-conflict protection is unchanged. A filtered empty result retains a visible reset action.
- The mobile editor does not automatically open the keyboard. Its fields scroll independently from the save/cancel footer. Desktop entry keeps text-field autofocus.
- Shared theme tokens, visible keyboard focus, accessible control names, native dialogs, and live save announcements remain in use. Success messages do not move the rows during repeated quantity adjustments.

## Verification

Run `scripts/inventory-check.cjs`, `scripts/inventory-materials-check.cjs`, and `scripts/inventory-ergonomics-check.cjs` only against the isolated integration-test preview; they create and delete test inventory. The preview and scripts default to `http://127.0.0.1:8099`. Production already uses port 8099.

To use a deliberately different test port, set `WORKSERVICE_UI_PORT` on the preview and `UI_BASE_URL` on browser tests. Start the compiled router test from `internal/router` with `WORKSERVICE_UI_PREVIEW=1` and `-test.run TestSiteIntegration/sqlite -test.v -test.timeout 0`. It uses temporary storage, never the working `storage` directory. Restart the preview after source or static-asset changes because it copies assets at startup.

Browser checks cover filtering, creation, quantity changes, draft retention, touch focus, light/dark themes, 320-1440px widths, long names, maximum quantities, keyboard actions, dialog scroll bounds and axe accessibility checks when `AXE_PATH` is supplied. Browser emulation is not a substitute for final physical iOS/Android keyboard testing.
