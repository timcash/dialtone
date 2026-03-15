package test

import "strings"

func (sc *StepContext) SetStatusPublisher(fn func(string, string)) {
	sc.logMu.Lock()
	sc.statusPublisher = fn
	pending := append([]stepStatusEvent{}, sc.pendingStatus...)
	sc.pendingStatus = nil
	sc.logMu.Unlock()
	if fn == nil {
		return
	}
	for _, ev := range pending {
		fn(ev.Kind, ev.Message)
	}
}

func (sc *StepContext) publishStatus(kind string, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "status"
	}
	sc.logMu.Lock()
	fn := sc.statusPublisher
	if fn == nil {
		sc.pendingStatus = append(sc.pendingStatus, stepStatusEvent{Kind: kind, Message: msg})
		sc.logMu.Unlock()
		return
	}
	sc.logMu.Unlock()
	fn(kind, msg)
}
