import { SectionComponent } from "../../util/ui";

export class TableSection implements SectionComponent {
  constructor(_container: HTMLElement) {}
  async mount() {
    console.log('Table mounted');
  }
  unmount() {}
  setVisible(_visible: boolean) {
    // Handled by SectionManager classes
  }
}