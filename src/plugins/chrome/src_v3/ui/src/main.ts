import { setupApp } from "../../../../ui/src_v1/ui/ui";
import {
  getUISharedTemplate,
  renderUISharedTemplate,
  type UISharedTemplateID,
} from "../../../../ui/src_v1/ui/templates";
import "../../../../ui/src_v1/ui/style.css";
import "./style.css";

type SectionEntry = {
  template: UISharedTemplateID;
  sectionID: string;
  title: string;
};

const sectionEntries: SectionEntry[] = [
  { template: "hero", sectionID: "chrome-home-stage", title: "Home" },
  { template: "docs", sectionID: "chrome-docs-docs", title: "Docs" },
];

const { sections, menu } = setupApp({
  title: "chrome src_v3 demo",
  debug: true,
});

for (const entry of sectionEntries) {
  const template = getUISharedTemplate(entry.template);
  sections.register(entry.sectionID, {
    containerId: entry.sectionID,
    canonicalName: entry.sectionID,
    load: async () => {
      const container = document.getElementById(entry.sectionID);
      if (!container) {
        throw new Error(`${entry.sectionID} container not found`);
      }
      renderUISharedTemplate(container, entry.template);
      decorateSection(entry, container);
      return {
        dispose: () => {},
        setVisible: (_visible: boolean) => {},
      };
    },
    overlays: template.overlays,
  });

  menu.addButton(entry.title, `Navigate ${entry.title}`, () => {
    void sections.navigateTo(entry.sectionID);
  });
}

void sections.navigateTo("chrome-home-stage");

function decorateSection(entry: SectionEntry, container: HTMLElement): void {
  if (container.dataset.enhanced === "1") {
    return;
  }
  container.dataset.enhanced = "1";

  if (entry.template === "hero") {
    const header = container.querySelector("header.overlay.legend dl");
    if (header) {
      const note = document.createElement("div");
      note.innerHTML = "<dt>server</dt><dd>vite :5173</dd>";
      header.appendChild(note);
    }
    const form = container.querySelector("form");
    if (form) {
      const hint = document.createElement("p");
      hint.className = "chrome-ui-hint";
      hint.textContent = "Use chrome src_v3 CLI commands to drive this browser tab.";
      form.prepend(hint);
    }
  }

  if (entry.template === "docs") {
    const article = container.querySelector("article");
    if (article) {
      article.innerHTML = `
        <h2>chrome src_v3 demo</h2>
        <p>This page is served by the Vite dev server in <code>src/plugins/chrome/src_v3/ui</code>.</p>
        <p>Use the CLI to open and retarget the single managed Chrome tab on the Windows host.</p>
        <pre>./dialtone.sh chrome src_v3 open --host legion --url http://127.0.0.1:5173
./dialtone.sh chrome src_v3 goto --host legion --url http://127.0.0.1:5173#chrome-docs-docs</pre>
      `;
    }
  }
}
