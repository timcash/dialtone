export const VisibilityMixin = {
  defaults: () => ({ isVisible: true, frameCount: 0 }),
  setVisible(target: { isVisible: boolean; frameCount: number }, visible: boolean, _name: string): void {
    target.isVisible = visible;
  },
};
