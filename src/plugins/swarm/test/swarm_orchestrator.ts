import { spawn } from "child_process";
import puppeteer from "puppeteer";
import fs from "fs";
import path from "path";

const LOG_FILE = path.join(process.cwd(), "e2e_unified.log");
const SCREENSHOT_DIR = path.join(process.cwd(), "screenshots");
const logStream = fs.createWriteStream(LOG_FILE, { flags: "w" });

function log(message: string) {
    const ts = new Date().toISOString();
    const line = `[${ts}] ${message}`;
    console.log(line);
    logStream.write(line + "\n");
}

function ensureScreenshotDir() {
    if (!fs.existsSync(SCREENSHOT_DIR)) {
        fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
    }
}

function screenshotPath(label: string) {
    const safeLabel = label.replace(/[^a-z0-9-_]+/gi, "_").toLowerCase();
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");
    return path.join(SCREENSHOT_DIR, `${stamp}_${safeLabel}.png`);
}

async function runTest() {
    log("Starting Consolidated Swarm E2E Test...");
    ensureScreenshotDir();

    // 1. Start the swarm dashboard
    log("Launching swarm dashboard via dialtone.sh...");
    const swarmProcess = spawn("./dialtone.sh", ["swarm", "dashboard"], {
        stdio: "pipe",
        env: { ...process.env, DEBUG: "*" }
    });

    swarmProcess.stdout.on("data", (data) => {
        log(`[SERVER OUT] ${data.toString().trim()}`);
    });

    swarmProcess.stderr.on("data", (data) => {
        log(`[SERVER ERR] ${data.toString().trim()}`);
    });

    log("Waiting 8s for server to settle...");
    await new Promise(r => setTimeout(r, 8000));

    // 2. Launch Puppeteer
    log("Connecting Puppeteer...");
    const browser = await puppeteer.launch({
        headless: true,
        args: ["--no-sandbox", "--disable-setuid-sandbox"]
    });

    try {
        const page = await browser.newPage();

        // 3. Inject Browser Logs, Errors, and Network Traffic
        page.on('console', msg => {
            log(`[BROWSER CONSOLE] ${msg.type().toUpperCase()}: ${msg.text()}`);
        });

        page.on('pageerror', err => {
            log(`[BROWSER ERROR] ${err.toString()}`);
        });

        page.on('request', request => {
            log(`[BROWSER REQUEST] ${request.method()} ${request.url()} (${request.resourceType()})`);
        });

        page.on('response', response => {
            log(`[BROWSER RESPONSE] ${response.status()} ${response.url()}`);
        });

        page.on('requestfailed', request => {
            const failure = request.failure();
            log(`[BROWSER REQUEST FAILED] ${request.url()} (${failure?.errorText || 'unknown error'})`);
        });

        log("Visiting http://127.0.0.1:4000...");
        await page.goto("http://127.0.0.1:4000", { waitUntil: "networkidle2", timeout: 15000 });
        await page.screenshot({ path: screenshotPath("loaded") });

        log("Initial load complete. Checking for nodes...");

        // Take a few screenshots and poll content
        for (let i = 0; i < 3; i++) {
            const content = await page.evaluate(() => document.body.innerText);
            log(`[POLL ${i}] Nodes found: ${content.includes('Running')}`);
            await page.screenshot({ path: screenshotPath(`poll_${i}`) });
            await new Promise(r => setTimeout(r, 3000));
        }

        const title = await page.title();
        log(`Final verification - Title: ${title}`);
        await page.screenshot({ path: screenshotPath("final") });

    } catch (err) {
        log(`TEST FAILURE: ${err.message}`);
    } finally {
        log("Tearing down...");
        await browser.close();
        swarmProcess.kill();
        log("Test run finished. Log written to e2e_unified.log");
    }
}

runTest().catch(console.error);
