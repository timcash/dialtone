import puppeteer from 'puppeteer';

async function runTest() {
    console.log('Starting Scroll Snap Test...');
    const browser = await puppeteer.launch({
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox']
    });
    const page = await browser.newPage();
    await page.setViewport({ width: 1280, height: 800 });

    try {
        console.log('Navigating to http://localhost:3000');
        await page.goto('http://localhost:3000', {
            waitUntil: 'networkidle0',
            timeout: 60000
        });

        // 1. Verify Slide 1: Globe
        console.log('Checking Slide 1: Globe');
        const globeExists = await page.evaluate(() => {
            const h1 = document.querySelector('h1');
            return h1?.innerText.includes('dialtone.earth');
        });
        if (!globeExists) throw new Error('Slide 1 (Globe) not found or title incorrect');

        // 2. Scroll to Slide 2: Video
        console.log('Scrolling to Slide 2');
        await page.evaluate(() => {
            const container = document.querySelector('.snap-container');
            if (container) {
                container.scrollTo({ top: window.innerHeight, behavior: 'auto' });
            }
        });
        await new Promise(r => setTimeout(r, 1000)); // Wait for snap/scroll

        const videoExists = await page.evaluate(() => {
            const h2 = Array.from(document.querySelectorAll('h2')).find(el => el.innerText === 'Robotic Operations');
            const video = document.querySelector('video');
            return !!h2 && !!video;
        });
        if (!videoExists) throw new Error('Slide 2 (Video) not found');

        // 3. Scroll to Slide 3: LineGraph
        console.log('Scrolling to Slide 3');
        await page.evaluate(() => {
            const container = document.querySelector('.snap-container');
            if (container) {
                container.scrollTo({ top: window.innerHeight * 2, behavior: 'auto' });
            }
        });
        await new Promise(r => setTimeout(r, 1000));

        const graphExists = await page.evaluate(() => {
            const h2 = Array.from(document.querySelectorAll('h2')).find(el => el.innerText === 'Neural Connectivity');
            const canvas = document.querySelector('canvas');
            return !!h2 && !!canvas;
        });
        if (!graphExists) throw new Error('Slide 3 (LineGraph) not found');

        // 4. Verify regular scrolling on About page via direct navigation
        console.log('Navigating to http://localhost:3000/about');
        await page.goto('http://localhost:3000/about', {
            waitUntil: 'networkidle0',
            timeout: 60000
        });

        await page.waitForSelector('h1');
        const aboutH1 = await page.evaluate(() => document.querySelector('h1')?.innerText);
        console.log(`About Page H1: ${aboutH1}`);

        const isSnapContainerPresent = await page.evaluate(() => {
            return !!document.querySelector('.snap-container');
        });
        if (isSnapContainerPresent) {
            console.log('Detected .snap-container on About page!');
            throw new Error('About page should not have .snap-container');
        }

        console.log('All scroll snap tests passed!');
    } catch (error) {
        console.error('Test failed:', error);
        process.exit(1);
    } finally {
        await browser.close();
    }
}

runTest();
