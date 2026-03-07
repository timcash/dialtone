const app = document.querySelector("#app");

app.innerHTML = `
  <main class="shell">
    <section class="hero">
      <p class="eyebrow">dialtone chrome/v1</p>
      <h1>Persistent Browser Demo</h1>
      <p class="lede">
        This page is served by a local Vite dev server and loaded through the
        chrome/v1 service using a persistent chromedp browser connection.
      </p>
    </section>
    <section class="card-row">
      <article class="card">
        <span class="label">Service</span>
        <strong>chrome/v1</strong>
        <p>One long-lived browser connection with NATS control.</p>
      </article>
      <article class="card">
        <span class="label">Tab</span>
        <strong>main</strong>
        <p>The service keeps a default tab ready for goto commands.</p>
      </article>
      <article class="card">
        <span class="label">Source</span>
        <strong>Vite</strong>
        <p>Hot-reload capable local dev UI from src/mods/chrome/v1/ui.</p>
      </article>
    </section>
  </main>
`;

const style = document.createElement("style");
style.textContent = `
  :root {
    color-scheme: light;
    --bg: #f4efe7;
    --ink: #171717;
    --accent: #e56b2f;
    --panel: rgba(255, 255, 255, 0.72);
    --line: rgba(23, 23, 23, 0.12);
  }

  * {
    box-sizing: border-box;
  }

  body {
    margin: 0;
    min-height: 100vh;
    font-family: Georgia, "Times New Roman", serif;
    color: var(--ink);
    background:
      radial-gradient(circle at top left, rgba(229, 107, 47, 0.18), transparent 28rem),
      radial-gradient(circle at bottom right, rgba(58, 90, 64, 0.18), transparent 24rem),
      linear-gradient(180deg, #f8f4ee, var(--bg));
  }

  .shell {
    max-width: 980px;
    margin: 0 auto;
    padding: 72px 24px 96px;
  }

  .hero {
    padding: 32px 0 20px;
  }

  .eyebrow {
    margin: 0 0 10px;
    text-transform: uppercase;
    letter-spacing: 0.18em;
    font: 600 12px/1.2 "Courier New", monospace;
    color: rgba(23, 23, 23, 0.56);
  }

  h1 {
    margin: 0;
    font-size: clamp(3rem, 7vw, 5.8rem);
    line-height: 0.95;
    letter-spacing: -0.04em;
  }

  .lede {
    max-width: 46rem;
    margin: 20px 0 0;
    font-size: clamp(1.05rem, 2vw, 1.35rem);
    line-height: 1.6;
  }

  .card-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 18px;
    margin-top: 42px;
  }

  .card {
    padding: 22px;
    border: 1px solid var(--line);
    border-radius: 20px;
    background: var(--panel);
    backdrop-filter: blur(10px);
    box-shadow: 0 20px 50px rgba(23, 23, 23, 0.07);
  }

  .label {
    display: block;
    margin-bottom: 12px;
    color: var(--accent);
    font: 700 11px/1.2 "Courier New", monospace;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .card strong {
    display: block;
    margin-bottom: 10px;
    font-size: 1.35rem;
  }

  .card p {
    margin: 0;
    line-height: 1.55;
  }
`;
document.head.appendChild(style);
