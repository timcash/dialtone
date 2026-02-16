package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"strings"
	"sync"
	"time"

	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/browser"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/performance"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
)

type sectionMetrics struct {
	CPU, Memory, GPU, JSHeap float64
	FPS                      int
	AppCPU, AppGPU           float64
}

func init() {
	test.Register("www-smoke", "www", []string{"www", "smoke", "browser"}, func() error {
		chromePath := browser.FindChromePath()
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck,
			chromedp.ExecPath(chromePath),
			chromedp.Headless,
		)
		allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
		defer allocCancel()
		return RunWwwSmokeSubTest(allocCtx)
	})
}

func RunWwwSmokeSubTest(allocCtx context.Context) error {
	fmt.Println(">> [WWW] Smoke: start")
	cwd, _ := os.Getwd()
	screenshotsDir := filepath.Join(cwd, "src", "plugins", "www", "screenshots")
	os.RemoveAll(screenshotsDir); os.MkdirAll(screenshotsDir, 0755)

	ctx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()

	// Initial setup with large timeout
	setupCtx, setupCancel := context.WithTimeout(ctx, 180*time.Second)
	defer setupCancel()

	var mu sync.Mutex
	performanceData := make(map[string]sectionMetrics)
	statsCh := make(chan sectionMetrics, 100)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if ce, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			msg := formatConsoleArgs(ce.Args)
			if strings.Contains(msg, "[SMOKE_STATS] ") {
				var s struct { FPS int; CPU, GPU float64 }
				json.Unmarshal([]byte(strings.TrimPrefix(msg, "[SMOKE_STATS] ")), &s)
				select { case statsCh <- sectionMetrics{FPS: s.FPS, AppCPU: s.CPU, AppGPU: s.GPU}: default: }
			}
			fmt.Printf("   [APP] %s\n", msg)
		}
	})

	var sections []string
	fmt.Println(">> [WWW] Smoke: setup")
	if err := chromedp.Run(setupCtx,
		performance.Enable(),
		chromedp.EmulateViewport(375, 812, chromedp.EmulateMobile),
		chromedp.Navigate("http://127.0.0.1:4173"),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(`(function(){
			const observer = new MutationObserver(()=>{
				const el = document.querySelector('.header-fps');
				if(!el || el.innerText.includes('FPS --')) return;
				const p = el.innerText.split('·');
				if(p.length>=3){
					console.log('[SMOKE_STATS] '+JSON.stringify({
						fps: parseInt(p[0].match(/: (\d+)/)?.[1]||0),
						cpu: parseFloat(p[1].match(/CPU ([\d\.]+) ms/)?.[1]||0),
						gpu: parseFloat(p[2].match(/GPU ([\d\.]+) ms/)?.[1]||0)
					}));
				}
			});
			observer.observe(document.querySelector('.header-fps'),{childList:true,characterData:true,subtree:true});
		})()`, nil),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('section[id^="s-"]')).map(el=>el.id)`, &sections),
	); err != nil {
		return fmt.Errorf("setup failed: %v", err)
	}

	for i, section := range sections {
		fmt.Printf(">> [WWW] Smoke: [%d/%d] #%s\n", i+1, len(sections), section)
		for len(statsCh) > 0 { <-statsCh }

		// Sub-test timeout: 5s
		subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
		
		var buf []byte
		var m sectionMetrics
		var jsM struct { Memory, JSHeap float64 }
		
		err := chromedp.Run(subCtx,
			chromedp.Evaluate(fmt.Sprintf("window.location.hash='%s'", section), nil),
			chromedp.Evaluate(`(async()=>({
				memory: performance.getEntriesByType('resource').reduce((a,r)=>a+(r.transferSize||0),0)/(1024*1024),
				jsHeap: (performance.memory?performance.memory.usedJSHeapSize:0)/(1024*1024)
			}))()`, &jsM),
			chromedp.ActionFunc(func(ctx context.Context) error {
				b, err := page.CaptureScreenshot().Do(ctx)
				buf = b
				return err
			}),
		)
		subCancel()
		
		if err != nil {
			fmt.Printf("   [ERROR] %s sub-test failed: %v\n", section, err)
			return err
		}

		select {
		case s := <-statsCh:
			m.FPS = s.FPS
			m.AppCPU = s.AppCPU
			m.AppGPU = s.AppGPU
		case <-time.After(2 * time.Second):
		}

		m.Memory, m.JSHeap = jsM.Memory, jsM.JSHeap
		mu.Lock(); performanceData[section] = m; mu.Unlock()
		if len(buf) > 0 { os.WriteFile(filepath.Join(screenshotsDir, section+".png"), buf, 0644) }
	}

	smokeMdPath := filepath.Join(cwd, "src", "plugins", "www", "SMOKE.md")
	report := "# Smoke Test Report\n\n| Section | FPS | CPU | GPU | Heap | Net |\n|---|---|---|---|---|---|\n"
	for _, s := range sections {
		d := performanceData[s]
		report += fmt.Sprintf("| %s | %d | %.2f | %.2f | %.2f | %.2f |\n", s, d.FPS, d.AppCPU, d.AppGPU, d.JSHeap, d.Memory)
	}
	os.WriteFile(smokeMdPath, []byte(report), 0644)
	TileScreenshots(screenshotsDir, filepath.Join(screenshotsDir, "summary.png"), sections)
	fmt.Println(">> [WWW] Smoke: pass"); return nil
}

func TileScreenshots(dir, out string, order []string) {
	var pngs []string
	for _, s := range order {
		p := filepath.Join(dir, s+".png")
		if _, err := os.Stat(p); err == nil { pngs = append(pngs, p) }
	}
	if len(pngs) == 0 { return }
	dst := image.NewRGBA(image.Rect(0, 0, 375*4, 812*rows(len(pngs), 4)))
	for i, p := range pngs {
		f, err := os.Open(p)
		if err != nil { continue }
		img, _, err := image.Decode(f)
		f.Close()
		if err == nil && img != nil {
			x, y := (i%4)*375, (i/4)*812
			draw.Draw(dst, image.Rect(x, y, x+375, y+812), img, image.Point{}, draw.Src)
		}
	}
	f, err := os.Create(out)
	if err == nil {
		png.Encode(f, dst)
		f.Close()
	}
}

func rows(n, cols int) int { return (n + cols - 1) / cols }

func formatConsoleArgs(args []*runtime.RemoteObject) string {
	var parts []string
	for _, a := range args {
		var val string
		if a.Value != nil {
			if err := json.Unmarshal(a.Value, &val); err == nil {
				parts = append(parts, val)
			} else {
				parts = append(parts, string(a.Value))
			}
		} else { parts = append(parts, a.Description) }
	}
	return strings.Join(parts, " ")
}

func waitForPortLocal(port int, timeout time.Duration) {
	start := time.Now()
	for time.Since(start) < timeout {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
			conn.Close(); return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err == nil { conn.Close(); return true }
	return false
}
