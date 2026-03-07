# chromedp Notes for chrome/v1

`chrome/v1` uses a single long-lived browser connection.

## Model

- start one Chrome instance with remote debugging enabled
- attach one allocator context with `chromedp.NewRemoteAllocator(...)`
- create one browser-level parent context with `chromedp.NewContext(allocCtx)`
- create one child context per managed tab

The service keeps the browser-level context alive for the full service lifetime.
That is the piece that should stay open continuously.

## Tab Behavior

- `chromedp.NewContext(parent)` creates a child context
- the first `chromedp.Run(childCtx, ...)` creates or attaches the tab target
- each managed tab keeps its own child context so later `goto` calls can reuse it

## Service Rule

The service should not tie its lifetime to a disposable tab context.
It should keep:
- one persistent browser connection
- one persistent `main` tab
- optional extra named tabs

## Sources

- https://pkg.go.dev/github.com/chromedp/chromedp#NewContext
- https://pkg.go.dev/github.com/chromedp/chromedp#NewRemoteAllocator
- https://pkg.go.dev/github.com/chromedp/chromedp#WithTargetID
