export class SettingsSection {
  constructor(_container: HTMLElement) {}
  async mount() { 
    console.log('Settings mounted'); 
  }
  unmount() {}
  setVisible(_visible: boolean) {
    // Handled by SectionManager classes
  }
}