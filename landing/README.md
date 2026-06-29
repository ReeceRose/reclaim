# Reclaim landing page

Public marketing site for [Reclaim](https://github.com/reecerose/reclaim). Deployed separately from the self-hosted app on Vercel.

## Local dev

```bash
cd landing
pnpm install
pnpm run dev
```

Open [http://localhost:3000](http://localhost:3000).

## Deploy to Vercel

1. Push this repo to GitHub.
2. In [Vercel](https://vercel.com/new), import the `reclaim` repository.
3. Set **Root Directory** to `landing`.
4. Deploy — Vercel auto-detects Next.js.

### Custom domain

Production site: **https://reclaim.reecerose.com**

1. In the Vercel project → **Settings → Domains**, add `reclaim.reecerose.com`.
2. Add the DNS record Vercel shows (usually a CNAME to `cname.vercel-dns.com`).

## Configuration

All links and copy live in `lib/site.ts`:

- `url` — canonical site URL (used in OG metadata)
- `repo` — GitHub repository
- `docker` — container image reference
