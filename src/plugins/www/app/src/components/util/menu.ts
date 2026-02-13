export class Menu {
    private static instance: Menu;
    private toggle: HTMLButtonElement;
    private panel: HTMLDivElement;


    private constructor() {
        // Bind to existing static elements
        const toggle = document.getElementById("global-menu-toggle") as HTMLButtonElement;
        const panel = document.getElementById("global-menu-panel") as HTMLDivElement;

        if (!toggle || !panel) {
            throw new Error("Global menu elements not found in DOM");
        }

        this.toggle = toggle;
        this.panel = panel;

        // Setup Toggle Logic
        const setOpen = (open: boolean) => {
            this.panel.hidden = !open;
            this.toggle.setAttribute("aria-expanded", String(open));
            
            // Disable background scrolling when menu is open
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
            setOpen(this.panel.hidden);
        });

        // Auto-close on scroll (only if scrolling the main page, not the menu itself)
        const onWindowScroll = (e: Event) => {
            if (this.panel.hidden) return;
            
            // If the scroll happened inside the menu panel, don't close it
            if (e.target === this.panel || this.panel.contains(e.target as Node)) {
                return;
            }
            
            setOpen(false);
        };
        window.addEventListener("scroll", onWindowScroll, { capture: true, passive: true });


        // Default state
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

    addSlider(
        label: string,
        value: number,
        min: number,
        max: number,
        step: number,
        onInput: (v: number) => void,
        format: (v: number) => string = (v) => v.toFixed(0)
    ) {
        const row = document.createElement("div");
        row.className = "menu-row";

        const labelWrap = document.createElement("label");
        const sliderId = `slider-${Math.random().toString(36).substr(2, 9)}`;
        labelWrap.className = "menu-label";
        labelWrap.htmlFor = sliderId;
        labelWrap.textContent = label;

        const slider = document.createElement("input");
        slider.type = "range";
        slider.className = "menu-input-range";
        slider.id = sliderId;
        slider.min = `${min}`;
        slider.max = `${max}`;
        slider.step = `${step}`;
        slider.value = `${value}`;

        const valueEl = document.createElement("span");
        valueEl.className = "menu-value";
        valueEl.textContent = format(value);

        slider.addEventListener("input", () => {
            const v = parseFloat(slider.value);
            onInput(v);
            valueEl.textContent = format(v);
        });

        row.appendChild(labelWrap);
        row.appendChild(slider);
        row.appendChild(valueEl);
        this.panel.appendChild(row);

        return {
            setValue: (v: number) => {
                slider.value = `${v}`;
                valueEl.textContent = format(v);
            }
        };
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

    addFile(label: string, onFile: (file: File) => void, accept = ".json,.geojson") {
        const row = document.createElement("div");
        row.className = "menu-row"; // Not really used for styling yet but consistency

        const input = document.createElement("input");
        input.type = "file";
        input.accept = accept;
        input.style.display = "none";

        const button = document.createElement("button");
        button.type = "button";
        button.className = "menu-button menu-button-primary";
        button.textContent = label;
        // button.style.width = "100%"; // Let CSS handle it

        button.addEventListener("click", () => input.click());
        input.addEventListener("change", () => {
            if (input.files?.[0]) {
                button.textContent = input.files[0].name;
                onFile(input.files[0]);
            }
        });

        this.panel.appendChild(button);
        this.panel.appendChild(input);
    }

    addStatus() {
        const row = document.createElement("div");
        row.className = "menu-row";
        const label = document.createElement("label");
        label.className = "menu-label";
        label.textContent = "Info";
        label.style.width = "40px";

        const value = document.createElement("span");
        value.className = "menu-value";
        value.style.width = "100%";
        value.style.textAlign = "left";
        value.style.opacity = "0.7";

        row.appendChild(label);
        row.appendChild(value);
        this.panel.appendChild(row);

        return {
            update: (text: string) => { value.textContent = text; }
        };
    }
}

