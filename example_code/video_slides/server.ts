import { ChromeService } from './chrome.js';
import puppeteer, { type Page } from 'puppeteer';
import indexHtml from './index.html';

const SERVER_PORT = 3000;
const CHROME_DEBUG_PORT = 9222;

class BunServer {
  private chromeService: ChromeService;
  private page: Page | null = null;
  private server: ReturnType<typeof Bun.serve> | null = null;

  constructor() {
    this.chromeService = new ChromeService();
  }

  async start() {
    console.log('🚀 Starting Bun server...');
    
    // Start Chrome service first
    await this.startChromeService();
    
    // Start the web server
    await this.startWebServer();
    
    // Navigate to the page with Puppeteer
    await this.navigateToPage();
    
    console.log('✅ Server setup completed!');
    console.log(`📱 Web server: http://localhost:${SERVER_PORT}`);
    console.log(`🔍 Chrome debug: http://localhost:${CHROME_DEBUG_PORT}`);
  }

  private async startChromeService() {
    console.log('🌐 Starting Chrome service...');
    await this.chromeService.start(CHROME_DEBUG_PORT);
    console.log(`✅ Chrome started on port ${CHROME_DEBUG_PORT}`);
  }

  private async startWebServer() {
    console.log('🌍 Starting web server...');
    
    this.server = Bun.serve({
      port: SERVER_PORT,
      routes: {
        '/': indexHtml,
        '/video1.mp4': {
          GET: () => {
            return new Response(Bun.file('./src/video1.mp4'));
          }
        },
        '/video2.mp4': {
          GET: () => {
            return new Response(Bun.file('./src/video2.mp4'));
          }
        },
        '/video3.mp4': {
          GET: () => {
            return new Response(Bun.file('./src/video3.mp4'));
          }
        },
        '/weld_tack_2.mp4': {
          GET: () => {
            return new Response(Bun.file('./src/weld_tack_2.mp4'));
          }
        }
      },
      development: {
        hmr: false,
        console: true,
      }
    });
    
    console.log(`✅ Web server started on port ${SERVER_PORT}`);
  }

  private async navigateToPage() {
    console.log('🎭 Connecting to Chrome and navigating to page...');
    
    try {
      // Connect to Chrome using Puppeteer
      this.page = await this.chromeService.newPage();
      
      // Navigate to our served HTML page
      console.log(`🌐 Navigating to http://localhost:${SERVER_PORT}...`);
      await this.page.goto(`http://localhost:${SERVER_PORT}`, {
        waitUntil: 'domcontentloaded',
        timeout: 10000
      });
      console.log('✅ Page navigation completed');
      
      // Set up additional monitoring
      this.setupPageMonitoring();
      
      // Tests will be run separately
      
    } catch (error) {
      console.error('❌ Error navigating to page:', error);
    }
  }

  private setupPageMonitoring() {
    if (!this.page) return;

    // Monitor page events
    this.page.on('load', () => {
      console.log('📄 Page loaded');
    });

    this.page.on('domcontentloaded', () => {
      console.log('📄 DOM content loaded');
    });

    // Monitor network requests with more detail
    this.page.on('request', (request) => {
      const resourceType = request.resourceType();
      const url = request.url();
      console.log(`🌐 [${resourceType.toUpperCase()}] ${request.method()} ${url}`);
    });

    this.page.on('response', (response) => {
      const url = response.url();
      const status = response.status();
      const statusText = response.statusText();
      const resourceType = response.request().resourceType();
      const size = response.headers()['content-length'] || 'unknown';
      
      console.log(`📡 [${resourceType.toUpperCase()}] ${status} ${statusText} ${url} (${size} bytes)`);
    });

    // Monitor failed requests
    this.page.on('requestfailed', (request) => {
      const url = request.url();
      const failure = request.failure();
      console.log(`❌ [FAILED] ${request.method()} ${url} - ${failure?.errorText || 'Unknown error'}`);
    });

    // Monitor page errors
    this.page.on('pageerror', (error) => {
      console.error('❌ Page error:', error.message);
    });

    // Monitor unhandled promise rejections
    this.page.on('unhandledrejection', (reason) => {
      console.error('❌ Unhandled promise rejection:', reason);
    });
  }

  async test() {
    if (!this.page) {
      console.log('❌ No page available for tests');
      return;
    }

    console.log('\n🧪 Starting video loading tests...');
    
    // Wait for initial page load
    console.log('⏳ Waiting 2 seconds for initial page load...');
    await new Promise(resolve => setTimeout(resolve, 2000));
    console.log('✅ Initial wait completed');
    
    // Check initial video loading state
    console.log('\n📊 Checking initial video loading state...');
    const initialVideoStates = await this.page.evaluate(() => {
      const videos = document.querySelectorAll('video');
      return Array.from(videos).map((video, index) => ({
        index: index + 1,
        src: video.src || video.querySelector('source')?.src || 'no src',
        readyState: video.readyState,
        networkState: video.networkState,
        paused: video.paused,
        currentTime: video.currentTime,
        duration: video.duration
      }));
    });

    console.log('Initial video states:');
    initialVideoStates.forEach(state => {
      console.log(`  Video ${state.index}: ${state.src.split('/').pop()} - ReadyState: ${state.readyState}, NetworkState: ${state.networkState}, Paused: ${state.paused}`);
    });

    // Verify only first 2 videos should be loading initially
    const firstTwoLoading = initialVideoStates.slice(0, 2).some(state => state.readyState > 0);
    const lastTwoNotLoading = initialVideoStates.slice(2).every(state => state.readyState === 0);
    
    console.log(`\n✅ First 2 videos loading: ${firstTwoLoading}`);
    console.log(`✅ Last 2 videos not loading: ${lastTwoNotLoading}`);

    // Scroll down to trigger lazy loading
    console.log('\n📜 Scrolling to trigger lazy loading...');
    
    // Scroll to section 3
    await this.page.evaluate(() => {
      const section3 = document.getElementById('section3');
      if (section3) {
        section3.scrollIntoView({ behavior: 'smooth' });
      }
    });
    
    // Wait for scroll to complete
    await new Promise(resolve => setTimeout(resolve, 1500));
    
    // Scroll to section 4
    await this.page.evaluate(() => {
      const section4 = document.getElementById('section4');
      if (section4) {
        section4.scrollIntoView({ behavior: 'smooth' });
      }
    });
    
    // Wait for videos to load after scroll
    await new Promise(resolve => setTimeout(resolve, 3000));
    
    // Check video loading state after scroll
    console.log('\n📊 Checking video loading state after scroll...');
    const afterScrollVideoStates = await this.page.evaluate(() => {
      const videos = document.querySelectorAll('video');
      return Array.from(videos).map((video, index) => ({
        index: index + 1,
        src: video.src || video.querySelector('source')?.src || 'no src',
        readyState: video.readyState,
        networkState: video.networkState,
        paused: video.paused,
        currentTime: video.currentTime,
        duration: video.duration
      }));
    });

    console.log('After scroll video states:');
    afterScrollVideoStates.forEach(state => {
      console.log(`  Video ${state.index}: ${state.src.split('/').pop()} - ReadyState: ${state.readyState}, NetworkState: ${state.networkState}, Paused: ${state.paused}`);
    });

    // Verify all videos are now loading
    const allVideosLoading = afterScrollVideoStates.every(state => state.readyState > 0);
    console.log(`\n✅ All videos loading after scroll: ${allVideosLoading}`);
    
    // Summary
    console.log('\n📋 Test Summary:');
    console.log(`  Initial state - First 2 loading: ${firstTwoLoading}`);
    console.log(`  Initial state - Last 2 not loading: ${lastTwoNotLoading}`);
    console.log(`  After scroll - All loading: ${allVideosLoading}`);
    
    if (firstTwoLoading && lastTwoNotLoading && allVideosLoading) {
      console.log('🎉 All tests passed! Lazy loading is working correctly.');
    } else {
      console.log('⚠️  Some tests failed. Check the video loading behavior.');
    }
    
    console.log('\n🏁 Test completed.');
  }

  async stop() {
    console.log('🛑 Stopping server...');
    
    if (this.server) {
      console.log('🌐 Stopping web server...');
      await this.server.stop();
    }
    
    // Disconnect from Chrome but keep it running
    console.log('🔌 Disconnecting from Chrome...');
    await this.chromeService.disconnect();
    
    console.log('✅ Server stopped (Chrome remains open)');
  }
}

// Main execution
async function main() {
  const server = new BunServer();
  
  // Check for CLI arguments
  const args = process.argv.slice(2);
  const runTests = args.includes('--test');
  
  // Handle graceful shutdown
  process.on('SIGINT', async () => {
    console.log('\n🛑 Received SIGINT, shutting down gracefully...');
    await server.stop();
    process.exit(0);
  });

  process.on('SIGTERM', async () => {
    console.log('\n🛑 Received SIGTERM, shutting down gracefully...');
    await server.stop();
    process.exit(0);
  });

  try {
    await server.start();
    
    if (runTests) {
      console.log('🧪 Running automated tests...');
      
      // Run tests
      await server.test();
      
      // Stop server
      await server.stop();
      
      console.log('✅ Process completed successfully');
      
      // Exit immediately after tests
      process.exit(0);
    } else {
      console.log('🌐 Server running. Press Ctrl+C to stop.');
      console.log('📄 Page available at: http://localhost:3000');
      
      // Keep the process running indefinitely
      await new Promise(() => {});
    }
    
  } catch (error) {
    console.error('❌ Failed to start server:', error);
    await server.stop();
    process.exit(1);
  }
}

// Run if this file is executed directly
if (import.meta.main) {
  main();
}
