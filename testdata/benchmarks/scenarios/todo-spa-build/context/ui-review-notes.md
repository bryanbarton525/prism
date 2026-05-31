# Prior UI review notes

Recent small-app reviews raised these issues repeatedly:

- Input fields without labels or aria descriptions.
- "Complete" actions that are mouse-only and not keyboard reachable.
- localStorage key collisions across demos.
- Missing empty-state guidance ("No todos yet").
- Styles that do not visibly distinguish completed items.

Requested fixes for future submissions:

- Add `<label>` for todo input.
- Ensure actionable controls are `<button>` elements.
- Use one explicit storage key, e.g. `todo-app.items.v1`.
- Render an empty-state message when list is empty.
- Toggle completed class with strike-through + muted color.
- Provide README test steps with expected behavior after reload.
