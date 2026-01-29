# Plugin: www

Vercel wrapper for the public website at [dialtone.earth](https://dialtone.earth).

## Folder Structure

```shell
src/plugins/www/
├── cli/
│   └── www.go           # CLI commands and Vercel integration
├── app/
│   ├── index.html       # Landing page with version tag
│   ├── package.json     # Version and dependencies
│   ├── vite.config.mjs  # Vite build config
│   ├── vercel.json      # Rewrites for /about, /docs
│   └── src/
│       ├── main.ts
│       ├── components/  # Earth, neural network, etc.
│       ├── pages/       # about.html, docs.html
│       └── shaders/     # GLSL shaders
└── README.md
```

## Command Line Help

```shell
./dialtone.sh www dev              # Start Vite dev server
./dialtone.sh www build            # Run local production build
./dialtone.sh www publish          # Bump version + build + deploy
./dialtone.sh www validate         # Check deployed version matches local
./dialtone.sh www logs <url>       # View deployment logs
./dialtone.sh www domain [url]     # Alias deployment to dialtone.earth
./dialtone.sh www login            # Login to Vercel
```

## Publish Workflow

```shell
# What ./dialtone.sh www publish does:
# 1. Bump patch version in package.json (1.0.9 → 1.0.10)
# 2. Update version tag in index.html (<p class="version">v1.0.10</p>)
# 3. Run npm run build (Vite production build)
# 4. Run vercel build --prod (create prebuilt output)
# 5. Run vercel deploy --prebuilt --prod
```

## Vercel Configuration

```shell
# Hardcoded in www.go:
VERCEL_PROJECT_ID=prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM
VERCEL_ORG_ID=team_4tzswM6M6PoDxaszH2ZHs5J7

# Project name in Vercel dashboard: app
# Domain: dialtone.earth
```

## Validation

```shell
# Verify deployed version matches local package.json
./dialtone.sh www validate

# Output:
# [www] Version OK: site=v1.0.9 expected=v1.0.9
```
