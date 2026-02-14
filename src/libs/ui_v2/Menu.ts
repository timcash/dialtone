export class Menu {
  private root: HTMLElement;
  private toggle: HTMLButtonElement;
  private items: HTMLButtonElement[] = [];

  constructor(rootSelector = '[aria-label="Global Menu"]') {
    const root = document.querySelector(rootSelector);
    if (!root) throw new Error('menu root not found');
    this.root = root as HTMLElement;

    const toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.textContent = 'Menu';
    toggle.classList.add('thumb');
    toggle.setAttribute('aria-label', 'Toggle Global Menu');
    toggle.setAttribute('aria-expanded', 'false');
    toggle.addEventListener('click', () => {
      const expanded = toggle.getAttribute('aria-expanded') === 'true';
      const nextExpanded = !expanded;
      this.setItemsVisible(nextExpanded);
      toggle.setAttribute('aria-expanded', String(nextExpanded));
    });
    this.root.appendChild(toggle);
    this.toggle = toggle;
  }

  addButton(label: string, ariaLabel: string, onClick: () => void): void {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.setAttribute('aria-label', ariaLabel);
    btn.setAttribute('data-menu-item', 'true');
    btn.hidden = true;
    btn.addEventListener('click', () => {
      onClick();
      this.setItemsVisible(false);
      this.toggle.setAttribute('aria-expanded', 'false');
    });
    this.items.push(btn);
    this.root.appendChild(btn);
  }

  private setItemsVisible(visible: boolean): void {
    for (const item of this.items) {
      item.hidden = !visible;
    }
  }
}
