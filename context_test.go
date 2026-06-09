// Copyright 2019 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	v8 "rogchap.com/v8go"
)

func TestContextExec(t *testing.T) {
	t.Parallel()
	ctx := v8.NewContext(nil)
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	ctx.RunScript(`const add = (a, b) => a + b`, "add.js")
	val, _ := ctx.RunScript(`add(3, 4)`, "main.js")
	rtn := val.String()
	if rtn != "7" {
		t.Errorf("script returned an unexpected value: expected %q, got %q", "7", rtn)
	}

	_, err := ctx.RunScript(`add`, "func.js")
	if err != nil {
		t.Errorf("error not expected: %v", err)
	}

	iso := ctx.Isolate()
	ctx2 := v8.NewContext(iso)
	defer ctx2.Close()
	_, err = ctx2.RunScript(`add`, "ctx2.js")
	if err == nil {
		t.Error("error expected but was <nil>")
	}
}

func TestJSExceptions(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		name   string
		source string
		origin string
		err    string
	}{
		{"SyntaxError", "bad js syntax", "syntax.js", "SyntaxError: Unexpected identifier 'js'"},
		{"ReferenceError", "add()", "add.js", "ReferenceError: add is not defined"},
	}

	ctx := v8.NewContext(nil)
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctx.RunScript(tt.source, tt.origin)
			if err == nil {
				t.Error("error expected but got <nil>")
				return
			}
			if err.Error() != tt.err {
				t.Errorf("expected %q, got %q", tt.err, err.Error())
			}
		})
	}
}

func TestContextRegistry(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	ctxref := ctx.Ref()

	c1 := v8.GetContext(ctxref)
	if c1 == nil {
		t.Error("expected context, but got <nil>")
	}
	if c1 != ctx {
		t.Errorf("contexts should match %p != %p", c1, ctx)
	}

	ctx.Close()

	c2 := v8.GetContext(ctxref)
	if c2 != nil {
		t.Error("expected context to be <nil> after close")
	}
}

func TestContextCloseFromDifferentGoroutine(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	done := make(chan struct{})
	go func() {
		ctx.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("context close timed out")
	}
}

func TestContextCloseWaitsForActiveFunctionCallback(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	entered := make(chan struct{})
	release := make(chan struct{})
	global := v8.NewObjectTemplate(iso)
	err := global.Set("block", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		close(entered)
		<-release
		return nil
	}))
	fatalIf(t, err)

	ctx := v8.NewContext(iso, global)
	runDone := make(chan error, 1)
	go func() {
		_, err := ctx.RunScript(`block()`, "block.js")
		runDone <- err
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("function callback was not entered")
	}

	closeDone := make(chan struct{})
	go func() {
		ctx.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		close(release)
		<-runDone
		t.Fatal("Context.Close returned before active function callback completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Context.Close did not return after function callback completed")
	}
}

func TestFunctionCallbackCanThrowAfterConcurrentCloseRequested(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	entered := make(chan struct{})
	release := make(chan struct{})
	global := v8.NewObjectTemplate(iso)
	err := global.Set("fail", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		close(entered)
		<-release
		msg, err := v8.NewValue(info.Context().Isolate(), "callback failed after close request")
		if err != nil {
			panic(err)
		}
		info.Context().Isolate().ThrowException(msg)
		return nil
	}))
	fatalIf(t, err)

	ctx := v8.NewContext(iso, global)
	runDone := make(chan error, 1)
	go func() {
		_, err := ctx.RunScript(`fail()`, "fail.js")
		runDone <- err
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("function callback was not entered")
	}

	closeDone := make(chan struct{})
	go func() {
		ctx.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		close(release)
		<-runDone
		t.Fatal("Context.Close returned before callback threw its exception")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	err = <-runDone
	if err == nil || !strings.Contains(err.Error(), "callback failed after close request") {
		t.Fatalf("expected callback exception, got %v", err)
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Context.Close did not return after callback exception")
	}
}

func TestMemoryLeak(t *testing.T) {
	t.Parallel()

	iso := v8.NewIsolate()
	defer iso.Dispose()

	for i := 0; i < 6000; i++ {
		ctx := v8.NewContext(iso)
		_ = ctx.Global()
		// _ = obj.String()
		_, _ = ctx.RunScript("2", "")
		ctx.Close()
	}
	if n := iso.GetHeapStatistics().NumberOfNativeContexts; n >= 6000 {
		t.Errorf("Context not being GC'd, got %d native contexts", n)
	}
}

// https://github.com/rogchap/v8go/issues/186
func TestRegistryFromJSON(t *testing.T) {
	t.Parallel()

	iso := v8.NewIsolate()
	defer iso.Dispose()

	global := v8.NewObjectTemplate(iso)
	err := global.Set("location", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		v, err := v8.NewValue(iso, "world")
		fatalIf(t, err)
		return v
	}))
	fatalIf(t, err)

	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	v, err := ctx.RunScript(`
		new Proxy({
			"hello": "unknown"
		}, {
			get: function () {
				return location()
			},
		})
	`, "main.js")
	fatalIf(t, err)

	s, err := v8.JSONStringify(ctx, v)
	fatalIf(t, err)

	expected := `{"hello":"world"}`
	if s != expected {
		t.Fatalf("expected %q, got %q", expected, s)
	}
}

func BenchmarkContext(b *testing.B) {
	b.ReportAllocs()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	for n := 0; n < b.N; n++ {
		ctx := v8.NewContext(iso)
		ctx.RunScript(script, "main.js")
		str, _ := json.Marshal(makeObject())
		cmd := fmt.Sprintf("process(%s)", str)
		ctx.RunScript(cmd, "cmd.js")
		ctx.Close()
	}
}

func ExampleContext() {
	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()
	ctx.RunScript("const add = (a, b) => a + b", "math.js")
	ctx.RunScript("const result = add(3, 4)", "main.js")
	val, _ := ctx.RunScript("result", "value.js")
	fmt.Println(val)
	// Output:
	// 7
}

func ExampleContext_isolate() {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx1 := v8.NewContext(iso)
	defer ctx1.Close()
	ctx1.RunScript("const foo = 'bar'", "context_one.js")
	val, _ := ctx1.RunScript("foo", "foo.js")
	fmt.Println(val)

	ctx2 := v8.NewContext(iso)
	defer ctx2.Close()
	_, err := ctx2.RunScript("foo", "context_two.js")
	fmt.Println(err)
	// Output:
	// bar
	// ReferenceError: foo is not defined
}

func ExampleContext_globalTemplate() {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	obj := v8.NewObjectTemplate(iso)
	obj.Set("version", "v1.0.0")
	ctx := v8.NewContext(iso, obj)
	defer ctx.Close()
	val, _ := ctx.RunScript("version", "main.js")
	fmt.Println(val)
	// Output:
	// v1.0.0
}
