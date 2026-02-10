export class TableSection {
  constructor(_container: HTMLElement) {}
  async mount() {
    console.log('Table mounted');
  }
  unmount() {}
  setVisible(_visible: boolean) {
    // Handled by SectionManager classes
  }
}
