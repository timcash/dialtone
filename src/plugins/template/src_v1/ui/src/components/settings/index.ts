import { SectionComponent } from "../../util/ui";

export class SettingsSection implements SectionComponent {
  constructor(_container: HTMLElement) {}
  async mount() { 
    console.log('Settings mounted'); 
  }
  unmount() {}
  setVisible(_visible: boolean) {
    // Handled by SectionManager classes
  }
}
