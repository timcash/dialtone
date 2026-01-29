# Plugin: www

Vercel wrapper for the public website.

## Commands
- `./dialtone.sh www dev`: start the Vite dev server
- `./dialtone.sh www build`: run the local production build
- `./dialtone.sh www publish`: bump patch version, build locally + `vercel build` + deploy with `--prebuilt`
- `./dialtone.sh www publish-prebuilt`: alias for `publish`
- `./dialtone.sh www validate`: fetch dialtone.earth and compare the UI version tag to `src/plugins/www/app/package.json`
- `./dialtone.sh www check-version`: alias for `validate`
- `./dialtone.sh www logs <deployment-url>`: view runtime logs
- `./dialtone.sh www domain [deployment-url]`: alias to `dialtone.earth`
