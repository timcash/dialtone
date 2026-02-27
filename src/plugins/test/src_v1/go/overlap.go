package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type OverlayOverlap struct {
	AKind         string  `json:"aKind"`
	AOverlay      string  `json:"aOverlay"`
	ARole         string  `json:"aRole"`
	ASection      string  `json:"aSection"`
	AAriaLabel    string  `json:"aAriaLabel"`
	BKind         string  `json:"bKind"`
	BOverlay      string  `json:"bOverlay"`
	BRole         string  `json:"bRole"`
	BSection      string  `json:"bSection"`
	BAriaLabel    string  `json:"bAriaLabel"`
	Intersection  float64 `json:"intersection"`
	PercentOfA    float64 `json:"percentOfA"`
	PercentOfB    float64 `json:"percentOfB"`
	AllowedByMenu bool    `json:"allowedByMenu"`
}

func (sc *StepContext) DetectOverlayOverlaps(timeout time.Duration) ([]OverlayOverlap, error) {
	const script = `(function() {
  const menuOpen = !!document.querySelector('nav[data-open="true"]');
  const overlays = Array.from(document.querySelectorAll('[data-overlay]')).filter((el) => {
    if (!(el instanceof HTMLElement)) return false;
    const cs = window.getComputedStyle(el);
    if (el.hidden) return false;
    if (cs.display === 'none' || cs.visibility === 'hidden' || cs.opacity === '0') return false;
    const r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    const isMenu = (el.getAttribute('data-overlay') || '') === 'menu';
    const isActive = (el.getAttribute('data-overlay-active') || '') === 'true';
    return isMenu || isActive;
  });
  const activeSection = document.querySelector('section[data-active="true"]');
  const buttons = Array.from(document.querySelectorAll('button')).filter((el) => {
    if (!(el instanceof HTMLButtonElement)) return false;
    const cs = window.getComputedStyle(el);
    if (el.hidden) return false;
    if (cs.display === 'none' || cs.visibility === 'hidden' || cs.opacity === '0') return false;
    const r = el.getBoundingClientRect();
    if (r.width <= 0 || r.height <= 0) return false;
    const inMenu = !!el.closest('[data-overlay="menu"]');
    if (inMenu) return menuOpen;
    const section = el.closest('section');
    if (!section) return false;
    if (!activeSection) return false;
    return section === activeSection;
  });

  function toRect(el) {
    const r = el.getBoundingClientRect();
    return { left: r.left, top: r.top, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
  }
  function area(r) { return Math.max(0, r.width) * Math.max(0, r.height); }
  function overlap(a, b) {
    const left = Math.max(a.left, b.left);
    const right = Math.min(a.right, b.right);
    const top = Math.max(a.top, b.top);
    const bottom = Math.min(a.bottom, b.bottom);
    if (right <= left || bottom <= top) return 0;
    return (right - left) * (bottom - top);
  }
  function info(el, kind) {
    const isButton = kind === 'button';
    const inMenu = !!el.closest('[data-overlay="menu"]');
    return {
      kind: kind,
      overlay: el.getAttribute('data-overlay') || '',
      role: el.getAttribute('data-overlay-role') || '',
      section: el.getAttribute('data-overlay-section') || '',
      aria: el.getAttribute('aria-label') || '',
      isNavButton: isButton && inMenu,
      menuOpen: menuOpen,
      rect: toRect(el),
    };
  }

  const infos = overlays.map((el) => info(el, 'overlay')).concat(buttons.map((el) => info(el, 'button')));
  const out = [];
  for (let i = 0; i < infos.length; i++) {
    for (let j = i + 1; j < infos.length; j++) {
      const a = infos[i];
      const b = infos[j];
      const inter = overlap(a.rect, b.rect);
      if (inter <= 0) continue;
      const aArea = area(a.rect);
      const bArea = area(b.rect);
      const allowedByMenu =
        a.overlay === 'menu' ||
        b.overlay === 'menu' ||
        ((a.isNavButton || b.isNavButton) && menuOpen);
      out.push({
        aKind: a.kind,
        aOverlay: a.overlay,
        aRole: a.role,
        aSection: a.section,
        aAriaLabel: a.aria,
        bKind: b.kind,
        bOverlay: b.overlay,
        bRole: b.role,
        bSection: b.section,
        bAriaLabel: b.aria,
        intersection: inter,
        percentOfA: aArea > 0 ? (inter / aArea) * 100 : 0,
        percentOfB: bArea > 0 ? (inter / bArea) * 100 : 0,
        allowedByMenu: allowedByMenu,
      });
    }
  }
  return JSON.stringify(out);
})()`

	var raw string
	if err := sc.RunBrowserWithTimeout(timeout, chromedp.Evaluate(script, &raw)); err != nil {
		return nil, fmt.Errorf("evaluate overlay overlap script: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []OverlayOverlap{}, nil
	}
	var overlaps []OverlayOverlap
	if err := json.Unmarshal([]byte(raw), &overlaps); err != nil {
		return nil, fmt.Errorf("decode overlay overlap output: %w", err)
	}
	return overlaps, nil
}
