# www_improvements

## Major Tasks
1. Migrate the `./diatone-earth` directory to the `./src/plugins/www/` directory
1. Factor out and Migrate the `dialtone-dev www` subcommand to the `./src/plugins/ww/cli` directory then connect it back to the main `dialtone-dev` cli tools
1. provide a `dialtone-dev www --help` command
1. provide a dev command for the `dialtone-dev www` subcommand that starts a local development server
1. inside `./src/plugins/www/test` create a `unit_test.go`, `integration_test.go`, and `e2e_test.go` for the `dialtone-dev www` subcommand

## Code guidlines
1. use simple HTML and TypeScript
2. use vite for serving the development server and building the distribution files

## Slide Pseudocode Example
```html
<html>
<head>
<style>
    article {
        width: 100vw;
        height: 100vh;
    }
</style>
</head>
<body>
<article>
    <header>
        <h1>Dialtone</h1>
    </header>
    <main>
        <p>Click to start</p>
    </main>
</article>
<article>
    <header>
        <h1>Dialtone</h1>
    </header>
    <main id="threejs_example"></main>
</article>
</body>
</html>
```


## Article List
1. `dialtone.earth` HERO Slide with call to action and spinning globe with blue blinking dots in the backgroud using three.js and globe.gl https://github.com/vasturiano/globe.gl
2. `dialtone.earth cli` Article with terminal like graphics in the background showing commands being run built with simple typescript and html
3. 


## Pages
1. Use `example_code/video_slides` as an example for the pages and how they should snap scroll and dynamically load content
1. All Pages should be a `slide` type which is a `<article>` with a `<header>` and `<main>`
1. `<article>` is always full 100vh and 100vw and has a fixed position
1. Add page for self improving AI system with list of issues from GitHub repo
1. Add page for CAD models of robots with three.js STL loader and wireframe fullscreen view
1. The site should integrate with vercel for deployment via the `dialtone-dev www deploy` command

## Page Tools
1. 


## NATS Connection
1. 