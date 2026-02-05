import { spawn } from "child_process";
import puppeteer from "puppeteer";
import fs from "fs";
import path from "path";

const LOG_FILE = path.join(process.cwd(), "e2e_unified.log");
const SCREENSHOT_PATH = path.join(process.cwd(), "dashboard_screenshot.png");
const logStream = fs.createWriteStream(LOG_FILE, { flags: "w" });

function log(message: string) {
    const ts = new Date().toISOString();
    const line = `[${ts}] ${message}`;
    console.log(line);
    logStream.write(line + "\n");
}

async function runTest() {
    log("Starting Consolidated Swarm E2E Test...");

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

        // 3. Inject Browser Logs & Errors
        page.on('console', msg => {
            log(`[BROWSER CONSOLE] ${msg.type().toUpperCase()}: ${msg.text()}`);
        });

        page.on('pageerror', err => {
            log(`[BROWSER ERROR] ${err.toString()}`);
        });

        log("Visiting http://127.0.0.1:4000...");
        await page.goto("http://127.0.0.1:4000", { waitUntil: "networkidle2", timeout: 15000 });

        log("Initial load complete. Checking for nodes...");

        // Take a few screenshots and poll content
        for (let i = 0; i < 3; i++) {
            const content = await page.evaluate(() => document.body.innerText);
            log(`[POLL ${i}] Nodes found: ${content.includes('Running')}`);
            await page.screenshot({ path: SCREENSHOT_PATH });
            await new Promise(r => setTimeout(r, 3000));
        }

        const title = await page.title();
        log(`Final verification - Title: ${title}`);

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
