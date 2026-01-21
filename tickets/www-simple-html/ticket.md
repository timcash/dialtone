# Branch: www-simple-html
# Task: Simplify WWW to HTML/TS with Vite

> IMPORTANT: Run `./dialtone.sh ticket start <this-file>` to start work.
> Run `./dialtone.sh github pull-request` to create a draft pull request

## Goals
1. Use tests files in `tickets/www-simple-html/test/` to drive all work.
2. use `./dialtone.sh plugin test www` to run those tests
3. Refactor `src/plugins/www/app` to use simple HTML, CSS, and TypeScript.
4. Remove Next.js and React dependencies while maintaining existing functionality.
5. Implement snap-scroll full-screen slides using `example_code/video_slides/index.html` as a reference.
6. Use simple libraries like d3.js for visualizations.
7. Use Vite for the development server and build process.
8. Integrate Vite into the `www dev` and `www build` CLI commands.

## Non-Goals
1. DO NOT use complex web frameworks (React, Next.js, Vue, etc.).
2. DO NOT lose existing D3 visualization functionality.

## Test
1. all ticket tests are at `tickets/www-simple-html/test/`
2. all plugin tests are run with `./dialtone.sh plugin test www`
3. all core tests are run with `./dialtone.sh test --core`
4. all tests are run with `./dialtone.sh test`

## Plugin Structure
1. `src/plugins/www/app` - Application code (HTML/CSS/TS/Vite).
2. `src/plugins/www/cli` - CLI command code (`www.go`).
3. `src/plugins/www/test` - Plugin-specific tests.
4. `src/plugins/www/README.md` - Plugin documentation.

## Subtask: Research
- description: Analyze current `src/plugins/www/app` to inventory functionality to be ported.
- description: Review `example_code/video_slides/index.html` for snap-scroll implementation details.
- status: todo

## Subtask: Scaffold & Test Setup
- description: Verify `tickets/www-simple-html/test/` exists (created by start command).
- description: [NEW] `tickets/www-simple-html/test/e2e_test.go`: Verify serving of static HTML files via Vite.
- status: todo

## Subtask: Implementation
- description: [MODIFY] `package.json`: Remove Next.js/React, add Vite and D3 dependencies.
- description: [DELETE] Remove `next.config.js` and `src/plugins/www/app` React files.
- description: [NEW] `src/plugins/www/app/index.html`: Create main HTML file with snap-scroll structure.
- description: [NEW] `src/plugins/www/app/style.css`: Implement CSS for snap scrolling and layout.
- description: [NEW] `src/plugins/www/app/main.ts`: Main TypeScript entry point.
- description: [NEW] `src/plugins/www/app/components/d3-globe.ts`: Port D3 globe functionality to vanilla TS/D3.
- description: [MODIFY] `src/plugins/www/cli/www.go`: Update `dev` command to run `vite` and `build` command to run `vite build`.
- status: todo

## Subtask: Verification
- description: Run test: `./dialtone.sh test`
- description: Manual verification of snap-scroll behavior in browser.
- description: Manual verification of D3 globe and other visualizations.
- status: todo

## Collaborative Notes
- We are replacing the Next.js stack with a "vanilla" web stack (HTML/CSS/TS) powered by Vite.
- CSS Scroll Snap will be the core mechanism for the slide layout.
- D3.js should be used for visualizations.
- `www dev` will wrapper specifically `vite` (or `npm run dev` which runs `vite`).
- `www build` will wrapper `vite build`.

---
Template version: 5.0. To start work: `./dialtone.sh ticket start <this-file>`
