export class Menu {
    private static instance: Menu;
    private toggle: HTMLButtonElement;
    private panel: HTMLDivElement;

    private constructor() {
        const toggle = document.getElementById("global-menu-toggle") as HTMLButtonElement;
        const panel = document.getElementById("global-menu-panel") as HTMLDivElement;
        
        if (!toggle || !panel) {
            console.warn("[Menu] Global menu elements not found, menu will be disabled");
            this.toggle = document.createElement("button");
            this.panel = document.createElement("div");
        } else {
            this.toggle = toggle;
            this.panel = panel;
        }

        const setOpen = (open: boolean) => {
            if (this.panel) this.panel.hidden = !open;
            if (this.toggle) this.toggle.setAttribute("aria-expanded", String(open));
        };

        if (toggle) {
            this.toggle.addEventListener("click", (e) => {
                e.preventDefault();
                setOpen(this.panel.hidden);
            });
        }

        window.addEventListener("scroll", () => { 
            if (this.panel && !this.panel.hidden) setOpen(false); 
        }, { capture: true, passive: true });

        setOpen(false);
    }

    public static getInstance(): Menu {
        if (!Menu.instance) Menu.instance = new Menu();
        return Menu.instance;
    }

    setVisible(visible: boolean) {
        const menuWrap = document.getElementById("global-menu");
        if (menuWrap) {
            menuWrap.classList.toggle("is-hidden", !visible);
        }
    }

    clear() { this.panel.innerHTML = ""; }

    addHeader(text: string) {
        const header = document.createElement("h3");
        header.className = "menu-header";
        header.textContent = text;
        this.panel.appendChild(header);
        return header;
    }

    addSlider(label: string, value: number, min: number, max: number, step: number, onInput: (v: number) => void, format: (v: number) => string = (v) => v.toFixed(0)) {
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
        return { setValue: (v: number) => { slider.value = `${v}`; valueEl.textContent = format(v); } };
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