export class Menu {
    private static instance: Menu;
    private toggle: HTMLButtonElement | null = null;
    private panel: HTMLDivElement | null = null;

    private constructor() {
        this.init();
    }

    private init() {
        if (typeof document === 'undefined') return;
        this.toggle = document.getElementById("global-menu-toggle") as HTMLButtonElement;
        this.panel = document.getElementById("global-menu-panel") as HTMLDivElement;

        if (!this.toggle || !this.panel) return;

        this.toggle.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            this.setOpen(this.panel?.hidden ?? true);
        });

        window.addEventListener("scroll", (e: Event) => {
            if (!this.panel || this.panel.hidden) return;
            if (e.target === this.panel || this.panel.contains(e.target as Node)) return;
            this.setOpen(false);
        }, { capture: true, passive: true });
    }

    private setOpen(open: boolean) {
        if (!this.panel || !this.toggle) return;
        
        if (open) {
            this.clear();
            window.dispatchEvent(new CustomEvent('menu-opening'));
        }

        this.panel.hidden = !open;
        this.toggle.setAttribute("aria-expanded", String(open));
        document.body.style.overflow = open ? 'hidden' : '';
    }

    public static getInstance(): Menu {
        if (!Menu.instance) Menu.instance = new Menu();
        return Menu.instance;
    }

    public isOpen(): boolean {
        return this.panel ? !this.panel.hidden : false;
    }

    public clear() {
        if (this.panel) this.panel.innerHTML = "";
    }

    public close() {
        this.setOpen(false);
    }

    addHeader(text: string) {
        if (!this.panel) return null;
        const el = document.createElement("h3");
        el.className = "menu-header";
        el.textContent = text;
        this.panel.appendChild(el);
        return el;
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

    addButton(label: string, onClick: () => void, active = false) {
        if (!this.panel) return document.createElement("button");
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = active ? "menu-button menu-button-active" : "menu-button";
        btn.textContent = label;
        btn.setAttribute("aria-label", label);
        btn.addEventListener("click", (e) => {
            e.stopPropagation();
            onClick();
        });
        this.panel.appendChild(btn);
        return btn;
    }

    addFile(label: string, onFile: (file: File) => void, accept = ".json,.geojson") {
        if (!this.panel) return;
        const input = document.createElement("input");
        input.type = "file";
        input.accept = accept;
        input.style.display = "none";

        const button = document.createElement("button");
        button.type = "button";
        button.className = "menu-button";
        button.textContent = label;
        button.setAttribute("aria-label", label);

        button.addEventListener("click", () => input.click());
        input.addEventListener("change", () => {
            if (input.files?.[0]) {
                onFile(input.files[0]);
            }
        });

        this.panel.appendChild(button);
        this.panel.appendChild(input);
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
