# Goal
create a golang cli tool to create template srcN folders with the minimal needed to run a simple autokv or autolog test that will pass and create the SMOKE.md file

# create a swarm/src_v2 ... 
- put all the code needed and package.json, warm.js, dashboard.html, index.js. only fill in the minimal needed to run a simple autokv or autolog test that will pass

# then create the minimal template for for a autokv and autolog test. 
- then add a `dialtone.sh swarm src --n N` where N will create or validate the srcN template folder. if it already exists it should just check the files and folders are there 

- use `src/plugins/swarm/cli` add a src.go file and put the cli code there. to make the srcN template folders with golang

# SMOKE.md
- see `src/plugins/www/test/smoke.go` to see how to create the test library it much use the `./diatone.sh chrome` pluging to create an manage chrome instances. the template test must show it can create the the `SMOKE.md` file and run the test.

# typescript CLI example for srcN command
- for the CLI do not use this code but use it as a reference. 
- the golang code should work and setup the files and folders
```typescript
import * as fs from 'fs';
import * as path from 'path';
import { execSync } from 'child_process';

const targetDir = process.argv[2];

if (!targetDir) {
    console.error("Usage: bun run template_src.ts <dir_name>");
    process.exit(1);
}

const root = path.resolve('.');
const srcPath = path.join(root, targetDir);
const testPath = path.join(srcPath, 'test');
const libPath = path.join(testPath, 'lib');

// 1. Create Directories
console.log('📂 Creating ' + targetDir + ' structure...');
[srcPath, testPath, libPath, path.join(srcPath, 'screenshots')].forEach(d => {
    if (!fs.existsSync(d)) fs.mkdirSync(d, { recursive: true });
});

// 2. Copy shared test utilities
console.log("🔗 Copying testing libraries...");
const sharedTestPath = path.join(root, 'src', 'test');
fs.copyFileSync(path.join(sharedTestPath, 'pixel_util.ts'), path.join(testPath, 'pixel_util.ts'));
fs.copyFileSync(path.join(sharedTestPath, 'lib', 'TestLibrary.ts'), path.join(libPath, 'TestLibrary.ts'));

// 3. Fix paths in copied TestLibrary
let libContent = fs.readFileSync(path.join(libPath, 'TestLibrary.ts'), 'utf8');
libContent = libContent.replace(/export const TEST_MD = 'src\/TEST\.md'/g, "export const TEST_MD = '" + targetDir + "/TEST.md'");
libContent = libContent.replace(/export const SCREENSHOTS_DIR = 'src\/screenshots'/g, "export const SCREENSHOTS_DIR = '" + targetDir + "/screenshots'");
fs.writeFileSync(path.join(libPath, 'TestLibrary.ts'), libContent);

// 4. Generate Domain Skeletons
console.log("🛠️ Generating domain skeletons...");
fs.writeFileSync(path.join(srcPath, 'Stage.ts'), 
'import * as THREE from "three";\n' +
'export class Stage {\n' +
'    scene = new THREE.Scene();\n' +
'    camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);\n' +
'    renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });\n' +
'    constructor() {\n' +
'        this.scene.background = new THREE.Color(0x111111);\n' +
'        this.renderer.setSize(window.innerWidth, window.innerHeight);\n' +
'        this.renderer.setClearColor(0x111111, 1);\n' +
'        const container = document.getElementById("canvas-container") || document.body;\n' +
'        container.appendChild(this.renderer.domElement);\n' +
'        this.scene.add(new THREE.AmbientLight(0xffffff, 0.8));\n' +
'        const dirLight = new THREE.DirectionalLight(0xffffff, 1);\n' +
'        dirLight.position.set(50, 50, 50);\n' +
'        this.scene.add(dirLight);\n' +
'    }\n' +
'    render() { this.renderer.render(this.scene, this.camera); }\n' +
'}');

fs.writeFileSync(path.join(srcPath, 'App.ts'), 
'import { Stage } from "./Stage";\n' +
'import * as THREE from "three";\n' +
'export class App {\n' +
'    stage = new Stage();\n' +
'    isV4Implemented = false;\n' +
'    constructor() {\n' +
'        const box = new THREE.Mesh(new THREE.BoxGeometry(5,5,5), new THREE.MeshPhongMaterial({color: 0x00ff00}));\n' +
'        this.stage.scene.add(box);\n' +
'        this.animate();\n' +
'    }\n' +
'    private animate() {\n' +
'        requestAnimationFrame(() => this.animate());\n' +
'        this.stage.render();\n' +
'    }\n' +
'    moveCamera(pos: THREE.Vector3, target: THREE.Vector3, alpha: number) {\n' +
'        // Implementation here\n' +
'    }\n' +
'}\n' +
'(window as any).app = new App();\n' +
'(window as any).THREE = THREE;');

// 5. Generate 20 Failing Test Templates
console.log("🧪 Generating 20 failing test templates...");
for (let i = 1; i <= 20; i++) {
    const fileName = (i < 10 ? '0' + i : i) + '_step.test.ts';
    const content = 
'import { TestLibrary, PORT } from "./lib/TestLibrary";\n' +
'import { PixelUtil } from "./pixel_util";\n\n' +
'async function run() {\n' +
'    const lib = new TestLibrary();\n' +
'    await lib.init();\n' +
'    try {\n' +
'        await lib.navigateTo("http://localhost:" + PORT + "/?t=" + Date.now());\n' +
'        lib.startStep("Step ' + i + ': Auto-generated Template");\n' +
'        \n' +
'        const isImplemented = await lib.page.evaluate(() => (window as any).app.isV4Implemented === true || (window as any).app.isV5Implemented === true);\n' +
'        if (!isImplemented) {\n' +
'            lib.log("Status: Implementation Pending");\n' +
'            await lib.snapshot("pending");\n' +
'            throw new Error("Step ' + i + ' not yet implemented by agent");\n' +
'        }\n\n' +
'        const centerPixel = await PixelUtil.getPixel(lib.page, 640, 360);\n' +
'        lib.log("Center Pixel: " + PixelUtil.colorToString(centerPixel));\n' +
'        \n' +
'        await lib.snapshot("verification");\n' +
'        await lib.finishStep("PASSED", "Verified step ' + i + ' ✅");\n' +
'    } catch (e) {\n' +
'        await lib.reportFailure(e);\n' +
'    } finally {\n' +
'        await lib.cleanup();\n' +
'        process.exit(0);\n' +
'    }\n' +
'}\n' +
'run();';
    fs.writeFileSync(path.join(testPath, fileName), content);
}

// 6. Generate run_all.ts
const testFilesList = [];
for(let i=1; i<=20; i++) testFilesList.push("    '" + targetDir + "/test/" + (i < 10 ? '0' + i : i) + "_step.test.ts'");

fs.writeFileSync(path.join(testPath, 'run_all.ts'), 
'import { execSync } from "child_process";\n' +
'import * as fs from "fs";\n' +
'const TEST_MD = "' + targetDir + '/TEST.md";\n' +
'const tests = [\n' +
testFilesList.join(',\n') + '\n' +
'];\n' +
'function prepare() {\n' +
'    fs.writeFileSync(TEST_MD, "# ' + targetDir + ' Results\\n\\nGenerated: " + new Date().toLocaleString() + "\\n\\n---\\n\\n");\n' +
'}\n' +
'console.log("🚀 Running ' + targetDir + ' suite...");\n' +
'prepare();\n' +
'for (const test of tests) {\n' +
'    console.log("\\n📂 Running: " + test);\n' +
'    try { execSync("bun run " + test, { stdio: "inherit" }); } catch (e) {}\n' +
'}\n');

// 7. Update package.json
console.log("📝 Updating package.json...");
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
pkg.scripts['build_' + targetDir] = 'bun build ./' + targetDir + '/App.ts --outdir ./dist --bundle';
pkg.scripts['test_' + targetDir] = 'bun run ./' + targetDir + '/test/run_all.ts';
fs.writeFileSync(path.join(root, 'package.json'), JSON.stringify(pkg, null, 2));

console.log('\n✅ ' + targetDir + ' scaffolded successfully!');

// --- Verification Step ---
console.log('🧪 Starting self-verification test for ' + targetDir + '...');

try {
    console.log('🔨 Building ' + targetDir + '...');
    execSync('bun run build_' + targetDir, { stdio: 'inherit' });

    console.log('🚀 Running initial test (expecting deliberate failure)...');
    // Note: The template test is designed to fail with "Implementation Pending"
    try {
        execSync('bun run ' + targetDir + '/test/01_step.test.ts', { stdio: 'inherit' });
    } catch (e) {
        console.log('ℹ️ Template test failed as expected (Step 1 not yet implemented).');
    }

    // Verify TEST.md exists and has content
    const testMdPath = path.join(srcPath, 'TEST.md');
    if (fs.existsSync(testMdPath)) {
        const content = fs.readFileSync(testMdPath, 'utf8');
        const hasLogs = content.includes('### Logs');
        const hasSnapshot = content.includes('failure_latest.png'); // Looking for failure snapshot
        const hasPending = content.includes('Implementation Pending');

        if (hasLogs && hasSnapshot && hasPending) {
            console.log('✅ TEST.md format verified successfully!');
        } else {
            console.error('❌ TEST.md content verification failed. Found snapshot: ' + hasSnapshot + ', Logs: ' + hasLogs + ', Pending: ' + hasPending);
            process.exit(1);
        }
    } else {
        console.error('❌ TEST.md was not generated.');
        process.exit(1);
    }

    console.log('\n✨ ' + targetDir + ' is ready for development!');
} catch (error) {
    console.error('❌ Verification failed: ', error);
    process.exit(1);
}
   ```


# testing library SMOKE.md: 
- use as example but we want golang and to the the `./dialtone.sh chrome` plugin
- we want our test system to output to the SMOKE.md file
- we want it to make patches not override the sections of SMOKE.md
```typescript
import puppeteer from 'puppeteer';
import { spawn, execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import { PixelUtil } from '../pixel_util';

export const TEST_MD = 'src5/TEST.md';
export const SCREENSHOTS_DIR = 'src5/screenshots';
export const PORT = 3001;

export class TestLibrary {
    logs: string[] = [];
    currentStep: string = "";
    page!: puppeteer.Page;
    browser!: puppeteer.Browser;
    serverProcess: any = null;
    snapshots: string[] = [];

    constructor() {
        if (!fs.existsSync(SCREENSHOTS_DIR)) {
            fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
        }
    }

    async init() {
        try { execSync(`lsof -ti:${PORT} | xargs kill -9`, { stdio: 'ignore' }); } catch(e) {}

        this.serverProcess = spawn('bun', ['run', 'dev'], { 
            shell: true,
            env: { ...process.env, PORT: PORT.toString() }
        });
        
        await new Promise(r => setTimeout(r, 2000));

        this.browser = await puppeteer.launch({ headless: "new" });
        this.page = await this.browser.newPage();
        await this.page.setViewport({ width: 1280, height: 720 });

        this.page.on('console', msg => this.log(`BROWSER: [${msg.type()}] ${msg.text()}`));
        this.page.on('pageerror', err => this.log(`PAGE ERROR: ${err.message}`));
    }

    startStep(name: string) {
        this.currentStep = name;
        this.logs = [];
        this.snapshots = [];
        const msg = "\n🚀 Starting Step: " + name;
        console.log(msg);
        this.logs.push(msg);
    }

    log(message: string) {
        const msg = "[" + new Date().toLocaleTimeString() + "] " + message;
        console.log(msg);
        this.logs.push(msg);
    }

    async snapshot(label: string) {
        const safeName = this.currentStep.toLowerCase().replace(/[^a-z0-9]/g, '_') + "_" + label.toLowerCase().replace(/[^a-z0-9]/g, '_');
        const screenshotPath = path.join(SCREENSHOTS_DIR, safeName + ".png");
        
        await this.page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        await this.page.screenshot({ path: screenshotPath });
        
        if (!this.snapshots.includes(safeName)) {
            this.snapshots.push(safeName);
        }
        this.log("📸 Snapshot captured: " + label);
    }

    async finishStep(status: 'PASSED' | 'FAILED', pixelData?: string) {
        if (this.snapshots.length === 0) {
            await this.snapshot('final');
        }

        const logsStr = this.logs.join('\n');
        let pixelMd = "";
        if (pixelData) {
            pixelMd = "### Pixel Verification\n" + pixelData + "\n\n";
        }
        
        let snapshotsMd = "### Screenshots\n";
        this.snapshots.forEach(s => {
            snapshotsMd += "![" + s + "](./screenshots/" + s + ".png)\n\n";
        });

        const mdEntry = "## " + this.currentStep + " (" + new Date().toLocaleTimeString() + ") - " + (status === 'PASSED' ? '✅' : '❌') + "\n\n" +
                        pixelMd +
                        "### Logs\n```\n" + logsStr + "\n```\n\n" +
                        snapshotsMd +
                        "---\n\n";
        
        this.updateMarkdown(mdEntry);
        console.log("🏁 Finished Step: " + this.currentStep + " (" + status + ")");
    }

    private updateMarkdown(mdEntry: string) {
        if (!fs.existsSync(TEST_MD)) {
            fs.writeFileSync(TEST_MD, mdEntry);
            return;
        }

        let content = fs.readFileSync(TEST_MD, 'utf8');
        const headerMarker = "## " + this.currentStep;
        
        // Find existing section
        const lines = content.split('\n');
        let startIndex = -1;
        let endIndex = -1;

        for (let i = 0; i < lines.length; i++) {
            if (lines[i].startsWith(headerMarker)) {
                startIndex = i;
                // Find next header or end
                for (let j = i + 1; j < lines.length; j++) {
                    if (lines[j].startsWith('## ')) {
                        endIndex = j;
                        break;
                    }
                }
                break;
            }
        }

        if (startIndex !== -1) {
            // Replace existing section
            const before = lines.slice(0, startIndex);
            const after = endIndex !== -1 ? lines.slice(endIndex) : [];
            content = [...before, mdEntry.trim(), ...after].join('\n');
        } else {
            // Append new section
            content = content.trim() + "\n\n---\n\n" + mdEntry;
        }

        fs.writeFileSync(TEST_MD, content);
    }

    async reportFailure(error: any) {
        this.log("❌ CRITICAL FAILURE: " + (error.message || error));
        
        const safeName = "failure_latest";
        const screenshotPath = path.join(SCREENSHOTS_DIR, safeName + ".png");
        if (this.page) await this.page.screenshot({ path: screenshotPath });

        const errorStack = error.stack || error;
        const logsStr = this.logs.join('\n');

        const mdEntry = "## 💥 Latest Failure - ❌\n\n" +
                        "### Error\n```\n" + errorStack + "\n```\n\n" +
                        "### Logs\n```\n" + logsStr + "\n```\n\n" +
                        "### Failure Screenshot\n![Failure](./screenshots/" + safeName + ".png)\n\n" +
                        "---\n\n";
        
        this.updateMarkdown(mdEntry);
    }

    async navigateTo(url: string) {
        this.log("Navigating to " + url + "...");
        await this.page.goto(url);
        await this.page.evaluate(() => {
            (window as any).DISABLE_LIVE_RELOAD = true;
            if ((window as any).socket) (window as any).socket.close();
        });
    }

    async cleanup() {
        if (this.browser) await this.browser.close().catch(() => {});
        if (this.serverProcess) this.serverProcess.kill('SIGKILL');
    }

    async logGeometry() {
        const geo = await this.page.evaluate(() => {
            const app = (window as any).app;
            const camera = app.stage.camera;
            const node = app.rootPlane.nodes.get('node_0');
            const worldPos = new (window as any).THREE.Vector3();
            if (node) node.mesh.getWorldPosition(worldPos);

            return {
                camera: {
                    pos: { x: camera.position.x, y: camera.position.y, z: camera.position.z },
                    rot: { x: camera.rotation.x, y: camera.rotation.y, z: camera.rotation.z }
                },
                node0: {
                    worldPos: { x: worldPos.x, y: worldPos.y, z: worldPos.z },
                    emissive: node ? node.mesh.material.emissive.getHex() : null,
                    intensity: node ? node.mesh.material.emissiveIntensity : null
                }
            };
        });
        this.log("GEOMETRY: Camera Pos: " + JSON.stringify(geo.camera.pos) + ", Rot: " + JSON.stringify(geo.camera.rot));
        this.log("GEOMETRY: Node_0 WorldPos: " + JSON.stringify(geo.node0.worldPos) + ", Emissive: 0x" + geo.node0.emissive?.toString(16) + ", Intensity: " + geo.node0.intensity);
    }
}
```