import { setupApp } from "../../../../ui/src_v1/ui/ui";
import {
  registerUISharedSections,
  type UISharedSectionEntry,
} from "../../../../ui/src_v1/ui/templates";
import "../../../../ui/src_v1/ui/style.css";
import "./style.css";

const sectionEntries: UISharedSectionEntry[] = [
  { template: "docs", sectionID: "chrome-home-docs", title: "Home" },
  { template: "table", sectionID: "chrome-runs-table", title: "Runs" },
  { template: "docs", sectionID: "chrome-docs-docs", title: "Docs" },
];

const { sections, menu } = setupApp({
  title: "chrome src_v3 demo",
  debug: true,
});

registerUISharedSections({
  sections,
  menu,
  entries: sectionEntries,
  decorate: (entry, container) => {
    decorateSection(entry, container);
  },
});

void sections.navigateTo("chrome-home-docs");

function decorateSection(entry: SectionEntry, container: HTMLElement): void {
  if (container.dataset.enhanced === "1") {
    return;
  }
  container.dataset.enhanced = "1";

  if (entry.sectionID === "chrome-home-docs") {
    const article = container.querySelector("article");
    const legendTitle = container.querySelector(".shell-legend-text h1");
    const legendText = container.querySelector(".shell-legend-text p");
    if (article) {
      article.innerHTML = `
        <h2>chrome src_v3 live demo</h2>
        <p>This page is served by the Vite dev server in <code>src/plugins/chrome/src_v3/ui</code>.</p>
        <p>Use the CLI to open and retarget the managed Chrome tab on the Windows host without direct browser attachment.</p>
        <pre>./dialtone.sh chrome src_v3 open --host legion --url http://127.0.0.1:5173/#chrome-home-docs
./dialtone.sh chrome src_v3 goto --host legion --url http://127.0.0.1:5173/#chrome-runs-table</pre>
      `;
    }
    if (legendTitle) legendTitle.textContent = "Managed browser demo";
    if (legendText) {
      legendText.textContent =
        "Simple docs-first landing section that matches the shared starter shell contract.";
    }
  }

  if (entry.sectionID === "chrome-runs-table") {
    const tbody = container.querySelector("tbody");
    const legend = container.querySelector(".shell-legend-telemetry dl");
    if (tbody) {
      tbody.innerHTML = `
        <tr><td>open</td><td>chrome.src_v3.dev.cmd</td><td>pass</td><td>41ms</td></tr>
        <tr><td>goto</td><td>managed-tab</td><td>pass</td><td>28ms</td></tr>
        <tr><td>get-url</td><td>managed-tab</td><td>pass</td><td>9ms</td></tr>
      `;
    }
    if (legend) {
      legend.innerHTML += `<div><dt>server</dt><dd>vite :5173</dd></div>`;
    }
  }
}
