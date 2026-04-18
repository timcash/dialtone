# CAD Publish Plan

Goal: give `cad src_v1` a real `publish` command that turns the CAD UI into a GitHub Pages PWA while wiring the live backend through a Cloudflare tunnel to this computer.

The desired operator story is:

```bash
./dialtone.sh cad src_v1 publish
```

That one command should:

- ensure the local CAD backend is running
- provision or reuse a stable Cloudflare tunnel hostname for that backend
- start the local `cloudflared` connector if needed
- build a GitHub Pages-ready CAD UI bundle that points at the live backend origin
- write a Pages artifact that the repo workflow can deploy
- print the live public URLs

## Public Targets

- GitHub Pages site:
  - `https://timcash.github.io/dialtone/cad-src-v1/`
- Cloudflare-backed CAD API origin:
  - `https://cad-src-v1.dialtone.earth`

The Pages site hosts the static PWA shell.
The Cloudflare hostname forwards to the local CAD Go server on this computer.

## Architecture

### Static Pages frontend

The Pages bundle must be static and portable:

- Vite base path is set to the repo Pages subpath
- the UI reads its backend base URL from build-time config
- a `manifest.webmanifest` and service worker make the published app installable and cache the shell
- the Pages output includes:
  - a landing `index.html`
  - the app under `cad-src-v1/`
  - `.nojekyll`
  - a `404.html` copy for SPA fallback

### Live backend

The CAD backend remains local:

- `cad src_v1 serve --port 8081` exposes the existing Go+Python API
- `publish` reuses a healthy backend when one is already running
- otherwise `publish` starts the backend in the background and records runtime state under `.dialtone/cad/src_v1/publish/`

### Cloudflare tunnel

Reuse the existing Dialtone Cloudflare runtime instead of inventing a second tunnel system:

- stable tunnel name:
  - `cad-src-v1`
- managed public hostname:
  - `cad-src-v1.dialtone.earth`
- store or reuse the tunnel run token in `env/dialtone.json`
- start a background `cloudflared tunnel run` connector for the local backend URL
- wait until the remote `/health` endpoint responds before declaring publish success

## Command contract

Add:

```bash
./dialtone.sh cad src_v1 publish
```

Recommended flags:

```bash
./dialtone.sh cad src_v1 publish --backend-port 8081
./dialtone.sh cad src_v1 publish --pages-only
./dialtone.sh cad src_v1 publish --backend-origin https://cad-src-v1.dialtone.earth
./dialtone.sh cad src_v1 publish --site-subpath cad-src-v1
```

Behavior:

- default `publish`
  - start or reuse backend
  - provision or reuse tunnel
  - start or reuse cloudflared
  - build Pages artifact
- `--pages-only`
  - skip local backend and tunnel startup
  - only build the Pages artifact from a known backend origin

## GitHub Pages workflow

Add a repo workflow similar to `cad-pga`:

- trigger on push to `main`
- set up Go
- set up Bun
- run the CAD publish command in Pages-only mode
- upload the generated Pages artifact
- deploy with `actions/deploy-pages`

The workflow should deploy the output directory produced by `cad src_v1 publish --pages-only`.

## Verification

Local verification:

```bash
./dialtone.sh cad src_v1 publish
./dialtone.sh cad src_v1 test --attach legion --filter cad-ui-browser-smoke
curl -I https://cad-src-v1.dialtone.earth/health
```

Pages verification:

```bash
gh run list --workflow "Deploy CAD Pages" --limit 5
curl -I https://timcash.github.io/dialtone/cad-src-v1/
```

## Done when

- `cad src_v1 publish` succeeds from WSL
- the Cloudflare hostname answers `/health`
- the GitHub Pages workflow deploys the CAD PWA
- the published site loads and can talk to the live backend through Cloudflare
