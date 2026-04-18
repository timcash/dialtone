package main

import "testing"

func TestShouldRunForegroundQueryCADTest(t *testing.T) {
	if !shouldRunForegroundQuery("cad", []string{"src_v1", "test"}) {
		t.Fatalf("cad src_v1 test should run in foreground")
	}
}

func TestShouldRunForegroundQueryCADPublish(t *testing.T) {
	if !shouldRunForegroundQuery("cad", []string{"src_v1", "publish"}) {
		t.Fatalf("cad src_v1 publish should run in foreground")
	}
}

func TestShouldRunForegroundQueryCADBuildStaysRouted(t *testing.T) {
	if shouldRunForegroundQuery("cad", []string{"src_v1", "build"}) {
		t.Fatalf("cad src_v1 build should stay routed")
	}
}

func TestShouldRouteCommandViaREPLHonorsForegroundCADTest(t *testing.T) {
	if shouldRouteCommandViaREPL("cad", []string{"src_v1", "test"}) {
		t.Fatalf("cad src_v1 test should not route via REPL")
	}
	if shouldRouteCommandViaREPL("cad", []string{"src_v1", "publish"}) {
		t.Fatalf("cad src_v1 publish should not route via REPL")
	}
	if !shouldRouteCommandViaREPL("cad", []string{"src_v1", "build"}) {
		t.Fatalf("cad src_v1 build should route via REPL")
	}
}
