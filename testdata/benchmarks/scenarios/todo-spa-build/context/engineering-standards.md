# Engineering standards excerpt

Frontend code submitted in this repo should bias toward:

- minimal dependencies for small widgets and demos,
- readable naming over clever abstractions,
- explicit empty/loading/error states where applicable,
- keyboard accessibility for primary actions,
- deterministic behavior under refresh/reload.

Review expectations:

- Keep functions small and focused.
- Isolate storage and rendering concerns.
- Keep DOM queries centralized.
- Avoid hidden global mutable state where possible.

Required output shape for this task:

Return file-by-file implementation snippets with exact headings:

1. `index.html`
2. `styles.css`
3. `app.js`
4. `README.md`

No additional framework boilerplate.
