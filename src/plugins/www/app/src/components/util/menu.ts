export class Menu {
    private static instance: Menu;
    private toggle: HTMLButtonElement | null = null;
    private panel: HTMLDivElement | null = null;
    private onOpenCallback: (() => void) | null = null;

    private constructor() {
        this.init();
    }

    private init() {
        if (typeof document === 'undefined') return;
        
        this.toggle = document.getElementById("global-menu-toggle") as HTMLButtonElement;
        this.panel = document.getElementById("global-menu-panel") as HTMLDivElement;

        if (!this.toggle || !this.panel) return;

        const setOpen = (open: boolean) => {
            if (!this.panel || !this.toggle) return;
            
            if (open && this.onOpenCallback) {
                this.clear();
                this.onOpenCallback();
            }

            this.panel.hidden = !open;
            this.toggle.setAttribute("aria-expanded", String(open));
            
            if (open) {
                document.body.style.overflow = 'hidden';
                document.documentElement.style.overflow = 'hidden';
            } else {
                document.body.style.overflow = '';
                document.documentElement.style.overflow = '';
            }
        };

        this.toggle.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            const nextState = this.panel?.hidden ?? true;
            setOpen(nextState);
        });

        const onWindowScroll = (e: Event) => {
            if (!this.panel || this.panel.hidden) return;
            if (e.target === this.panel || this.panel.contains(e.target as Node)) return;
            setOpen(false);
        };
        window.addEventListener("scroll", onWindowScroll, { capture: true, passive: true });
    }

    public static getInstance(): Menu {
        if (!Menu.instance) {
            Menu.instance = new Menu();
        }
        return Menu.instance;
    }

    public onOpen(cb: () => void) {
        this.onOpenCallback = cb;
    }

    clear() {
        if (this.panel) this.panel.innerHTML = "";
    }

    close() {
        if (!this.panel || !this.toggle) return;
        this.panel.hidden = true;
        this.toggle.setAttribute("aria-expanded", "false");
        document.body.style.overflow = '';
        document.documentElement.style.overflow = '';
    }

    addHeader(text: string) {
        if (!this.panel) return null;
        const header = document.createElement("h3");
        header.className = "menu-header";
        header.textContent = text;
        this.panel.appendChild(header);
        return header;
    }

    addSlider(label: string, value: number, min: number, max: number, step: number, onInput: (v: number) => void, format: (v: number) => string = (v) => v.toFixed(0)) {
        if (!this.panel) return { setValue: () => {} };
        const row = document.createElement("div");
        row.className = "menu-row";
        const labelEl = document.createElement("label");
        labelEl.className = "menu-label";
        labelEl.textContent = label;
        
        const slider = document.createElement("input");
        slider.type = "range";
        slider.className = "menu-input-range";
        slider.min = String(min);
        slider.max = String(max);
        slider.step = String(step);
        slider.value = String(value);
        slider.setAttribute("aria-label", label);

        const valueEl = document.createElement("span");
        valueEl.className = "menu-value";
        valueEl.textContent = format(value);

        slider.addEventListener("input", () => {
            const v = parseFloat(slider.value);
            onInput(v);
            valueEl.textContent = format(v);
        });

        row.appendChild(labelEl);
        row.appendChild(slider);
        row.appendChild(valueEl);
        this.panel.appendChild(row);

        return { setValue: (v: number) => { slider.value = String(v); valueEl.textContent = format(v); } };
    }

    addButton(label: string, onClick: () => void, primary = false) {
        if (!this.panel) return document.createElement("button");
        const button = document.createElement("button");
        button.type = "button";
        button.className = primary ? "menu-button menu-button-primary" : "menu-button";
        button.textContent = label;
        button.setAttribute("aria-label", label);
        button.addEventListener("click", onClick);
        this.panel.appendChild(button);
        return button;
    }

    addStatus() {
        if (!this.panel) return { update: () => {} };
        const row = document.createElement("div");
        row.className = "menu-row";
        const label = document.createElement("label");
        label.className = "menu-label";
        label.textContent = "Info";
        const value = document.createElement("span");
        value.className = "menu-value";
        value.setAttribute("aria-label", "status-info");
        row.appendChild(label);
        row.appendChild(value);
        this.panel.appendChild(row);
        return { update: (text: string) => { value.textContent = text; } };
    }
}
