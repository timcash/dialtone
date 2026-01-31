# Accessibility and Keyboard-Driven Testing

I have implemented full accessibility coverage for the marketing sections and transitioned the testing suite to a keyboard-driven model.

## Changes Made

### Accessibility (A11y)
- **Aria Labels**: Added descriptive `aria-label` attributes to all sections, marketing overlays, equations (math-snippets), and the Stripe payment container in `index.html`.
- **Text & Labels**: Ensured every interactive and informational element has both visible text and an accessible label for screen readers.

### Navigation and UX
- **Keyboard Navigation**: Implemented a global "Space bar" listener in `main.ts`. Users can now cycle through the snap-scroll slides by pressing Space.
- **Auto-Loop**: The Space bar navigation wraps back to the first slide after the final slide (Stripe offer).

### Robust Verification
- **Keyboard-Driven Tests**: Refactored `test.go` to use `chromedp.KeyEvent(" ")` for navigating sections during verification.
- **Aria-Label Selectors**: Updated test logic to locate elements using their `aria-label`. This ensures the UI is both functional and accessible.
- **Optimized Browser Setup**: Maintained the high-performance browser logic that reuses existing Chrome debugger instances to prevent system bloat.

## Verification Results

### Subtask Status
All tests passed using the new keyboard-driven logic:
- `add-marketing-text`: [PASS] (Verified via 'Unified Networks marketing information' label)
- `add-robot-kit-offer`: [PASS] (Verified via 'Order section: Robot Kit offer' label)
- `verify-browser`: [PASS] (Verified via keyboard cycling and 'Precision Control marketing information' label)

### Terminal Snapshot
```
[dialtest] Running test for subtask: add-marketing-text
[dialtest] PASS: add-marketing-text
[dialtest] Running test for subtask: add-robot-kit-offer
[dialtest] PASS: add-robot-kit-offer
[dialtest] Running test for subtask: verify-browser
[dialtest] PASS: verify-browser
[ticket] Tests passed!
```
