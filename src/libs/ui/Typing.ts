export function startTyping(
  subtitleEl: HTMLParagraphElement | null,
  subtitles: string[],
  options: { typeMs?: number; holdMs?: number; fadeMs?: number } = {}
) {
  if (!subtitleEl || subtitles.length === 0) {
    return () => {};
  }
  const typeMs = options.typeMs ?? 42;
  const holdMs = options.holdMs ?? 6000;
  const fadeMs = options.fadeMs ?? 1200;
  let index = 0;
  let charIndex = 0;
  let typingTimer: number | undefined;
  let typingTimeout: number | undefined;
  subtitleEl.style.opacity = "1";
  subtitleEl.style.transition = `opacity ${fadeMs}ms ease`;

  const step = () => {
    const full = subtitles[index];
    const next = full.slice(0, Math.min(full.length, charIndex + 1));
    subtitleEl.textContent = `| ${next || "\u00A0"}`;
    charIndex += 1;
    if (charIndex >= full.length) {
      typingTimeout = window.setTimeout(() => {
        subtitleEl.style.opacity = "0";
        typingTimeout = window.setTimeout(() => {
          index = (index + 1) % subtitles.length;
          charIndex = 0;
          subtitleEl.textContent = "| \u00A0";
          subtitleEl.style.opacity = "1";
          step();
        }, fadeMs);
      }, holdMs);
      return;
    }
    typingTimer = window.setTimeout(step, typeMs);
  };

  step();

  return () => {
    if (typingTimer) window.clearTimeout(typingTimer);
    if (typingTimeout) window.clearTimeout(typingTimeout);
  };
}
