import { setupApp } from "../../../../ui/src_v1/ui/ui";
import { registerUISharedSections } from "../../../../ui/src_v1/ui/templates";
import "../../../../ui/src_v1/ui/style.css";
import "./style.css";

const { sections, menu } = setupApp({
  title: "Dialtone Earth",
  debug: true,
  pwa: {
    enabled: true,
    serviceWorkerPath: "/sw.js",
    disableInDev: false,
  },
});

const SECTION_ID_HERO = "earth-hero-stage";

registerUISharedSections({
  sections,
  menu,
  entries: [{ sectionID: SECTION_ID_HERO, template: "three", title: "Hero" }],
  decorate: async (entry, container) => {
    if (entry.sectionID === SECTION_ID_HERO) {
      sections.setLoadingMessage(SECTION_ID_HERO, "loading earth hero...");
      const { mountHero } = await import("./components/hero/index");
      const canvas = container.querySelector("canvas");
      if (!canvas) throw new Error("three canvas not found in template");
      return mountHero(container);
    }
  },
});

const hash = window.location.hash.slice(1).trim();
const initial = hash === "" || hash === "hero" ? SECTION_ID_HERO : hash;
void sections.navigateTo(initial);
