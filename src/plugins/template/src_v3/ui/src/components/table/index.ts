import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class TableControl implements VisualizationControl {
  private page = 1;
  private onPrev: () => void;
  private onNext: () => void;

  constructor(private container: HTMLElement) {
    const pageEl = container.querySelector("[aria-label='Table Page']") as HTMLElement | null;
    const prevEl = container.querySelector("button[aria-label='Table Pagination Prev']") as HTMLButtonElement | null;
    const nextEl = container.querySelector("button[aria-label='Table Pagination Next']") as HTMLButtonElement | null;

    if (!pageEl || !prevEl || !nextEl) {
      throw new Error('table controls not found');
    }

    const render = () => {
      pageEl.textContent = `Page ${this.page}`;
      prevEl.disabled = this.page <= 1;
    };

    this.onPrev = () => {
      if (this.page > 1) this.page -= 1;
      render();
    };
    this.onNext = () => {
      this.page += 1;
      render();
    };

    prevEl.addEventListener('click', this.onPrev);
    nextEl.addEventListener('click', this.onNext);
    render();
  }

  dispose(): void {
    const prevEl = this.container.querySelector("button[aria-label='Table Pagination Prev']") as HTMLButtonElement | null;
    const nextEl = this.container.querySelector("button[aria-label='Table Pagination Next']") as HTMLButtonElement | null;
    prevEl?.removeEventListener('click', this.onPrev);
    nextEl?.removeEventListener('click', this.onNext);
  }

  setVisible(_visible: boolean): void {}
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
