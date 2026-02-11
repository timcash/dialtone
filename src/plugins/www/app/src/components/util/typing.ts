export function startTyping(
  subtitleEl: HTMLParagraphElement | null,
  subtitles: string[],
  options: { holdMs?: number; fadeMs?: number } = {}
) {
  if (!subtitleEl || subtitles.length === 0) {
    return () => {};
  }
  const holdMs = options.holdMs ?? 5000;
  const fadeMs = options.fadeMs ?? 250;
  let index = 0;
  let swapTimer: number | undefined;
  let fadeTimer: number | undefined;
  subtitleEl.style.opacity = "1";
  subtitleEl.style.transition = `opacity ${fadeMs}ms ease`;
  subtitleEl.textContent = subtitles[index];

  const swap = () => {
    subtitleEl.style.opacity = "0";
    fadeTimer = window.setTimeout(() => {
      index = (index + 1) % subtitles.length;
      subtitleEl.textContent = subtitles[index];
      subtitleEl.style.opacity = "1";
    }, fadeMs);
  };
  swapTimer = window.setInterval(swap, holdMs);

  return () => {
    if (swapTimer) window.clearInterval(swapTimer);
    if (fadeTimer) window.clearTimeout(fadeTimer);
  };
}
