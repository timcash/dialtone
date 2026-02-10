export class Menu {
  private static instance: Menu;
  private toggle: HTMLButtonElement;
  private panel: HTMLDivElement;

  private constructor() {
    const toggle = document.getElementById("global-menu-toggle") as HTMLButtonElement;
    const panel = document.getElementById("global-menu-panel") as HTMLDivElement;

    if (!toggle || !panel) {
      throw new Error("Global menu elements not found in DOM");
    }

    this.toggle = toggle;
    this.panel = panel;

    const setOpen = (open: boolean) => {
      this.panel.hidden = !open;
      this.toggle.setAttribute("aria-expanded", String(open));
    };

    this.toggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(this.panel.hidden);
    });

    const onWindowScroll = () => {
      if (!this.panel.hidden) setOpen(false);
    };
    window.addEventListener("scroll", onWindowScroll, { capture: true, passive: true });

    setOpen(false);
  }

  public static getInstance(): Menu {
    if (!Menu.instance) {
      Menu.instance = new Menu();
    }
    return Menu.instance;
  }

  clear() {
    this.panel.innerHTML = "";
  }

  addHeader(text: string) {
    const header = document.createElement("h3");
    header.className = "menu-header";
    header.textContent = text;
    this.panel.appendChild(header);
    return header;
  }

  addButton(label: string, onClick: () => void, primary = false) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = primary ? "menu-button menu-button-primary" : "menu-button";
    button.textContent = label;
    button.addEventListener("click", onClick);
    this.panel.appendChild(button);
    return button;
  }
}
