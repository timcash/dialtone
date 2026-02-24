package repl

import (
	"fmt"
	"io"
	"sync"
)

type joinConsole struct {
	mu          sync.Mutex
	out         io.Writer
	prompt      string
	interactive bool
}

func newJoinConsole(out io.Writer, prompt string, interactive bool) *joinConsole {
	return &joinConsole{
		out:         out,
		prompt:      prompt,
		interactive: interactive,
	}
}

func (c *joinConsole) PrintFrame(frame BusFrame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearActivePrompt()
	printFrame(c.out, frame)
	c.renderPrompt()
}

func (c *joinConsole) PrintLine(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearActivePrompt()
	fmt.Fprintln(c.out, msg)
	c.renderPrompt()
}

func (c *joinConsole) Prompt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	fmt.Fprintf(c.out, "%s> ", c.prompt)
}

func (c *joinConsole) clearActivePrompt() {
	if !c.interactive {
		return
	}
	fmt.Fprint(c.out, "\r\033[K")
}

func (c *joinConsole) renderPrompt() {
	if !c.interactive {
		return
	}
	fmt.Fprintf(c.out, "%s> ", c.prompt)
}
