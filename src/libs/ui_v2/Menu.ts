export class Menu {
  private panel: HTMLElement;

  constructor(panelSelector = '[aria-label="Global Menu Panel"]') {
    const panel = document.querySelector(panelSelector);
    if (!panel) throw new Error('menu panel not found');
    this.panel = panel as HTMLElement;
  }

  addButton(label: string, ariaLabel: string, onClick: () => void): void {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = label;
    btn.setAttribute('aria-label', ariaLabel);
    btn.addEventListener('click', onClick);
    this.panel.appendChild(btn);
  }
}
