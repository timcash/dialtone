export class Menu {
  private root: HTMLElement;
  private toggle: HTMLButtonElement;
  private panel: HTMLDivElement;
  private grid: HTMLDivElement;
  private items: HTMLButtonElement[] = [];
  private readonly onDocumentClick: (event: MouseEvent) => void;
  private readonly onEscape: (event: KeyboardEvent) => void;

  constructor(rootSelector = '[aria-label="Global Menu"]') {
    const root = document.querySelector(rootSelector);
    if (!root) throw new Error('menu root not found');
    this.root = root as HTMLElement;
    this.root.setAttribute('data-overlay', 'menu');

    const existingPanel = this.root.querySelector('[aria-label="Global Menu Panel"]');
    let panel: HTMLDivElement;
    if (existingPanel instanceof HTMLDivElement) {
      panel = existingPanel;
    } else {
      panel = document.createElement('div');
      panel.setAttribute('aria-label', 'Global Menu Panel');
      panel.setAttribute('role', 'dialog');
      panel.setAttribute('aria-modal', 'true');
      panel.hidden = true;
      this.root.appendChild(panel);
    }
    panel.classList.add('menu-panel');
    panel.setAttribute('role', 'dialog');
    panel.setAttribute('aria-modal', 'true');
    this.panel = panel;
    let grid: HTMLDivElement | null = this.panel.querySelector('.menu-grid');
    if (!(grid instanceof HTMLDivElement)) {
      grid = document.createElement('div');
      grid.classList.add('menu-grid');
      this.panel.appendChild(grid);
    }
    this.grid = grid;

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.textContent = 'Menu';
    toggle.classList.add('thumb');
    toggle.setAttribute('aria-label', 'Toggle Global Menu');
    toggle.setAttribute('aria-expanded', 'false');
    toggle.classList.add('menu-toggle');
    toggle.addEventListener('click', () => {
      this.setOpen(!this.isOpen());
    });
    this.root.appendChild(toggle);
    this.toggle = toggle;

    this.onDocumentClick = (event: MouseEvent) => {
      if (!this.isOpen()) return;
      const target = event.target as Node | null;
      if (!target) return;
      if (target === this.panel) {
        this.setOpen(false);
        return;
      }
      if (this.root.contains(target)) return;
      this.setOpen(false);
    };
    this.onEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        this.setOpen(false);
      }
    };

    document.addEventListener('click', this.onDocumentClick, { capture: true });
    window.addEventListener('keydown', this.onEscape);
    this.setOpen(false);
  }

  addButton(label: string, ariaLabel: string, onClick: () => void): void {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.classList.add('menu-button');
    btn.setAttribute('aria-label', ariaLabel);
    btn.setAttribute('data-menu-item', 'true');
    btn.hidden = true;
    btn.addEventListener('click', () => {
      onClick();
      this.setOpen(false);
    });
    this.items.push(btn);
    this.grid.appendChild(btn);
  }

  private setItemsVisible(visible: boolean): void {
    for (const item of this.items) {
      item.hidden = !visible;
    }
  }

  private isOpen(): boolean {
    return this.toggle.getAttribute('aria-expanded') === 'true';
  }

  private setOpen(open: boolean): void {
    this.setItemsVisible(open);
    this.panel.hidden = !open;
    this.toggle.setAttribute('aria-expanded', String(open));
    document.body.classList.toggle('menu-open', open);
    document.body.style.overflow = open ? 'hidden' : '';
    document.documentElement.style.overflow = open ? 'hidden' : '';
  }
}
