export class SettingsSection {
  constructor(private container: HTMLElement) {}
  async mount() { console.log('Settings mounted'); }
  unmount() {}
  setVisible(visible: boolean) {
    this.container.style.display = visible ? 'block' : 'none';
  }
}
