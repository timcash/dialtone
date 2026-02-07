export class Menu {


    private toggle: HTMLButtonElement;
    private panel: HTMLDivElement;
    private cleanupScroll: () => void;



    constructor(containerId: string, title = "Menu") {
        // 1. Create Toggle Button
        const controls = document.querySelector(".top-right-controls");
        if (!controls) throw new Error("Could not find .top-right-controls");

        this.toggle = document.createElement("button");
        this.toggle.className = "menu-toggle";
        this.toggle.type = "button";
        this.toggle.setAttribute("aria-expanded", "false");
        this.toggle.textContent = title;
        // Prepend to put it before any existing toggles (though usually only one exists per section)
        controls.prepend(this.toggle);

        // 2. Create/Find Panel
        let panel = document.getElementById(containerId) as HTMLDivElement | null;
        if (!panel) {
            // If not found, create it (backwards compatibility or new standard)
            panel = document.createElement("div");
            panel.id = containerId;
            // Assume it should be appended to the app container or similar?
            // For now, we expect it to exist in the HTML as per current pattern, 
            // or we throw if strict. The current pattern is container.innerHTML = ... <div id="...">
            // So we might just fail if it's not there yet.
            // Let's assume it exists for now as it's usually part of the component mount.
            console.warn(`Panel #${containerId} not found in DOM at Menu creation time.`);
        }
        this.panel = panel!;
        this.panel.classList.add("menu-panel");
        this.panel.hidden = true;

        // 3. Setup Toggle Logic
        const setOpen = (open: boolean) => {
            this.panel.hidden = !open;
            this.panel.style.display = open ? "grid" : "none";
            this.toggle.setAttribute("aria-expanded", String(open));
        };

        this.toggle.addEventListener("click", (e) => {
            e.preventDefault();
            e.stopPropagation();
            setOpen(this.panel.hidden);
        });

        // 4. Auto-close on scroll
        const onWindowScroll = () => {
            if (!this.panel.hidden) setOpen(false);
        };
        window.addEventListener("scroll", onWindowScroll, { capture: true, passive: true });
        this.cleanupScroll = () => {
            window.removeEventListener("scroll", onWindowScroll, { capture: true } as any);
        };

        // Default state
        setOpen(false);

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

        // If not primary, maybe wrap in a row? Or just append? 
        // Current design mixes rows and direct buttons. 
        // Let's checking styling. Premium buttons usually full width or grouped.
        // For now, append directly.
        this.panel.appendChild(button);
        return button;
    }

    addFile(label: string, onFile: (file: File) => void, accept = ".json,.geojson") {
        const row = document.createElement("div");
        row.className = "menu-row";

        const input = document.createElement("input");
        input.type = "file";
        input.accept = accept;
        input.style.display = "none";

        const button = document.createElement("button");
        button.type = "button";
        button.className = "menu-button menu-button-primary";
        button.textContent = label;
        button.style.width = "100%"; // Fill row if inside row? Or just stand alone?
        // In geotools it was a row with button.

        button.addEventListener("click", () => input.click());
        input.addEventListener("change", () => {
            if (input.files?.[0]) {
                button.textContent = input.files[0].name;
                onFile(input.files[0]);
            }
        });

        // Maybe just append button directly if we want full width? 
        // But we might want a label?
        // Geotools had a "Data Source" header then the button.
        this.panel.appendChild(button); // Direct append for full width
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

    setToggleVisible(visible: boolean) {
        this.toggle.hidden = !visible;
        this.toggle.style.display = visible ? "inline-block" : "none";
    }

    dispose() {
        this.cleanupScroll();
        this.toggle.remove();
        // We don't remove the panel as it's often part of the mount HTML,
        // but the component disposal usually clears the container anyway.
    }
}
