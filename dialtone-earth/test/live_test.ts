import puppeteer from 'puppeteer';

async function runTest() {
    const browser = await puppeteer.launch({
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox']
    });
    const page = await browser.newPage();

    try {
        console.log('Navigating to Home Page...');
        await page.goto('https://dialtone-ten.vercel.app/', {
            waitUntil: 'domcontentloaded',
            timeout: 60000
        });

        // Verify Title
        const title = await page.title();
        console.log(`Page Title: ${title}`);
        if (title !== 'dialtone.earth') {
            throw new Error(`Unexpected title: ${title}`);
        }

        // Verify main heading or text
        const textContent = await page.evaluate(() => document.body.innerText);
        if (!textContent.includes('dialtone.earth')) {
            throw new Error('Home page does not contain "dialtone.earth"');
        }
        console.log('Home Page verified.');

        // Navigate to About Page
        console.log('Navigating to About Page...');
        await page.click('a[href="/about"]');

        // Wait for the specific element on the new page instead of waitForNavigation
        await page.waitForSelector('h1', { timeout: 30000 });

        // Verify About Page Content
        const aboutTitle = await page.evaluate(() => document.querySelector('h1')?.innerText);
        console.log(`About Page Title: ${aboutTitle}`);
        if (aboutTitle !== 'Vision') {
            throw new Error(`Unexpected About page title: ${aboutTitle}`);
        }
        console.log('About Page verified.');

        console.log('All tests passed successfully!');
    } catch (error) {
        console.error('Test failed:', error);
        process.exit(1)
    } finally {
        await browser.close();
    }
}

runTest();
