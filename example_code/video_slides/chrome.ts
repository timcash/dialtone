import { spawn } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import puppeteer, { type Browser, type Page } from 'puppeteer';

const REMOTE_DEBUG_PORT = 9222;
const USER_DATA_DIR = path.join(process.cwd(), 'chrome-debug-data');

export class ChromeService {
  private browser: Browser | null = null;
  public remoteDebugPort: number = REMOTE_DEBUG_PORT;
  private chromeProcess: ReturnType<typeof spawn> | null = null;

  constructor() {}

  async start(remoteDebugPort: number = REMOTE_DEBUG_PORT): Promise<void> {
    if (this.browser?.connected) {
      return;
    }

    this.remoteDebugPort = remoteDebugPort;

    // Try to connect to existing Chrome instance first
    try {
      this.browser = await puppeteer.connect({
        browserURL: `http://localhost:${this.remoteDebugPort}`
      });
      
      return;
    } catch (error) {
      // Continue to create new Chrome instance
    }

    await fs.promises.mkdir(USER_DATA_DIR, { recursive: true });

    if (this.browser?.connected) {
      this.browser.disconnect();
    }
    this.browser = null;

    const chromePath = await this.findChromeExecutable();
    if (!chromePath) {
      throw new Error('Chrome executable not found in standard locations');
    }

    const args = [
      `--remote-debugging-port=${this.remoteDebugPort}`,
      `--user-data-dir=${USER_DATA_DIR}`,
      '--no-first-run',
      '--no-default-browser-check'
    ];

    this.chromeProcess = spawn(chromePath, args, {
      detached: true,
      stdio: 'ignore'
    });

    // Wait for Chrome to be ready
    this.browser = await this.waitForChromeReady();
  }

  async newPage(): Promise<Page> {
    if (!this.browser?.connected) {
      throw new Error('Chrome is not connected');
    }

    // use the first open page
    const pages = await this.browser.pages();
    if (pages.length === 0) {
      throw new Error('No open pages found');
    }
    const page = pages[0];
    if (!page) {
      throw new Error('No open pages found');
    }
    this.setupConsoleMonitoring(page);
    return page;
  }

  async disconnect(): Promise<void> {
    if (!this.browser) {
      return;
    }

    this.browser.disconnect();
  }

  async stop(): Promise<void> {
    if (this.browser) {
      try {
        // close all pages
        const pages = await this.browser.pages();
        for (const page of pages) {
          await page.close();
        }
        this.browser.disconnect();
      } catch (error) {
        // Ignore errors
      } finally {
        this.browser = null;
      }
    }
  }

  private getChromePaths(): string[] {
    const platform = os.platform();

    if (platform === 'win32') {
      return [
        path.join(process.env.ProgramFiles || '', 'Google', 'Chrome', 'Application', 'chrome.exe'),
        path.join(process.env['ProgramFiles(x86)'] || '', 'Google', 'Chrome', 'Application', 'chrome.exe')
      ];
    }

    if (platform === 'darwin') {
      return [
        '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
        '/Applications/Chromium.app/Contents/MacOS/Chromium'
      ];
    }

    return [
      '/usr/bin/google-chrome',
      '/usr/bin/chromium',
      '/usr/bin/google-chrome-stable'
    ];
  }

  private async findChromeExecutable(): Promise<string | null> {
    const candidates = this.getChromePaths();

    for (const chromePath of candidates) {
      try {
        await fs.promises.access(chromePath, fs.constants.F_OK);
        return chromePath;
      } catch {
        // continue to next candidate
      }
    }

    return null;
  }

  private async waitForChromeReady(): Promise<Browser> {
    const timeout = 5000;
    const retryInterval = 250;
    const deadline = Date.now() + timeout;
    let lastError: unknown;

    while (Date.now() < deadline) {
      let browser: Browser | null = null;

      try {
        browser = await puppeteer.connect({
          browserURL: `http://localhost:${this.remoteDebugPort}`
        });

        return browser;
      } catch (error) {
        lastError = error;

        if (browser) {
          browser.disconnect();
        }

        await new Promise(resolve => setTimeout(resolve, retryInterval));
      }
    }

    throw new Error(`Timed out waiting for Chrome to become available${lastError ? `: ${lastError}` : ''}`);
  }

  private setupConsoleMonitoring(page: Page): void {
    page.on('console', (msg) => {
      const type = msg.type();
      const text = msg.text();
      const location = msg.location();
      let locationStr = 'unknown';

      // if (msg.stackTrace()) {
      //   const stackTrace = msg.stackTrace();
      //   for (const frame of stackTrace) {
      //     console.log(frame);
      //   }
      // }

      if (location.url) {
        locationStr = location.url;
        if (location.lineNumber !== undefined) {
          locationStr += `:${location.lineNumber}`;
          if (location.columnNumber !== undefined) {
            locationStr += `:${location.columnNumber}`;
          }
        }
      }

      const timestamp = new Date().toISOString();
      const logMessage = `[${timestamp}] [${type.toUpperCase()}] ${text} (${locationStr})`;
      console.log(logMessage);
    });
  }
}

// CLI functionality
async function main() {
  const service = new ChromeService();
  
  try {
    console.log('Starting Chrome in debug mode...');
    await service.start();
    console.log(`Chrome started on port ${service.remoteDebugPort}`);
    console.log('Chrome is running. Press Ctrl+C to stop.');
    
    // Keep the process alive
    process.on('SIGINT', async () => {
      console.log('\nShutting down Chrome...');
      await service.stop();
      process.exit(0);
    });
    
    // Keep the process running indefinitely
    await new Promise(() => {});
    
  } catch (error) {
    console.error('Error:', error);
    process.exit(1);
  }
}

// Run CLI if this file is executed directly
if (import.meta.main) {
  main();
}
