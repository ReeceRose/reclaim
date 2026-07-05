<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

# UI components

Always prefer ShadCN components (`/components/ui/`) over custom HTML elements. Specifically:

- Use `<Button>` instead of `<button>` — pick the closest variant (`default`, `outline`, `ghost`, `destructive`, `secondary`, `link`) and override with `className` only for brand-gradient or one-off styles.
- Use `<Badge>` instead of inline `<span>` pills/tags — pass `className` to override shape, color, or size when the built-in variants don't fit.
- Use `<Select>` / `<SelectTrigger>` / `<SelectContent>` / `<SelectItem>` instead of `<select>` / `<option>`. Radix Select requires non-empty string values — use a sentinel like `'all'` for "unset" states and convert in `onValueChange`.
- Use `<Input>` instead of `<input>` (including `type="time"`, `type="number"`, etc.) — pass `className` for width or rounding overrides.
- Use `<Checkbox>` instead of custom checkbox `<span>` elements. Supports `checked={true | false | 'indeterminate'}`.
- Use `<Dialog>` / `<DialogContent>` / `<DialogHeader>` / `<DialogTitle>` / `<DialogFooter>` for modals.

The project CSS maps `--primary → --brand` and `--primary-foreground → --on-brand`, so ShadCN's default color tokens align with the design system automatically. Do not reimplement what ShadCN already provides.

# Loading states

Always use `<Suspense>` + `<Skeleton>` for data loading — never plain text ("Loading…") or conditional renders that hide content during fetch.

- Split data-fetching pages into an inner `PageContent` component that uses `useSuspenseQuery` (or `useSuspenseInfiniteQuery`) and an outer `Page` shell that wraps it in `<Suspense fallback={<PageSkeleton />}>`.
- `<Skeleton>` is at `/components/ui/skeleton`. It uses `animate-pulse` and `bg-accent` (which maps to `--surface-3` in this theme) — no className override needed for the default dark look.
- Design skeleton fallbacks to match the real layout's structure and approximate dimensions (cards, rows, bars) so there's no layout shift on load.
- For sub-components that fetch independently (e.g. a grouped list view), wrap them in their own `<Suspense>` with a local skeleton rather than relying on the parent's boundary.
- For virtualised/infinite lists that can't easily use `useSuspenseInfiniteQuery`, gate on `data === undefined` (pre-first-fetch) and render skeleton rows in place of the virtual scroller.

# web/out/.gitkeep

Never delete or stage the deletion of `web/out/.gitkeep`. `web/out/*` is gitignored except this file (see root `.gitignore`), which keeps the directory present in git so `go:embed all:out` compiles before a frontend build has ever run. `pnpm run build` clears `web/out/` before regenerating the static export and does not recreate this file — if you run a build, check `git status` for a deletion of `web/out/.gitkeep` before committing or staging broadly, and restore it if gone.
