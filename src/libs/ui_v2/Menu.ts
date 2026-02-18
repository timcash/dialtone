export class Menu {
  private root: HTMLElement;
  private toggle: HTMLButtonElement;
  private items: HTMLButtonElement[] = [];
  private readonly onDocumentClick: (event: MouseEvent) => void;
  private readonly onEscape: (event: KeyboardEvent) => void;

  constructor(rootSelector = '[aria-label="Global Menu"]') {
    const root = document.querySelector(rootSelector);
    if (!root) throw new Error('menu root not found');
    this.root = root as HTMLElement;
    this.root.setAttribute('data-overlay', 'menu');

    const existingPanel = this.root.querySelector('[aria-label="Global Menu Panel"]');
    if (existingPanel instanceof HTMLDivElement) {
      existingPanel.hidden = true;
      existingPanel.remove();
    }

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.textContent = 'Menu';
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
      if (target === this.root) {
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
    this.root.appendChild(btn);
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
    this.toggle.setAttribute('aria-expanded', String(open));
    this.root.setAttribute('data-open', open ? 'true' : 'false');
    this.root.setAttribute('role', open ? 'dialog' : 'navigation');
    this.root.setAttribute('aria-modal', open ? 'true' : 'false');
    document.body.classList.toggle('menu-open', open);
    document.body.style.overflow = open ? 'hidden' : '';
    document.documentElement.style.overflow = open ? 'hidden' : '';
  }
}
