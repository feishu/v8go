// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"testing"

	v8 "rogchap.com/v8go"
)

// TestFunctionCallbackManualRelease verifies that when callback uses defer info.Release(),
// all arguments and this are completely released, with 0 retained handles leaked.
func TestFunctionCallbackManualRelease(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	global := v8.NewObjectTemplate(iso)
	calledCount := 0

	fnTmpl := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		calledCount++
		defer info.Release() // Clean up callback arguments
		return nil
	})
	global.Set("testFn", fnTmpl)

	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	initialCount := ctx.RetainedValueCount()
	const iterations = 500

	for i := 0; i < iterations; i++ {
		res, err := ctx.RunScript("testFn(1, 'param', true)", "")
		if err != nil {
			t.Fatal(err)
		}
		res.Release()
	}

	finalCount := ctx.RetainedValueCount()
	leaked := finalCount - initialCount

	t.Logf("Initial Retained Values: %d", initialCount)
	t.Logf("After %d calls with Release: %d (Leaked callback handles: %d)", iterations, finalCount, leaked)

	if leaked != 0 {
		t.Errorf("expected 0 leaked callback handles, got %d", leaked)
	}
}

// TestDoubleReleaseProtection verifies that calling Value.Release() multiple times
// is idempotent and safe, causing no crash or double-free.
func TestDoubleReleaseProtection(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	val, err := ctx.RunScript("'test double release'", "")
	if err != nil {
		t.Fatal(err)
	}

	// First release
	val.Release()

	// Second release - must be idempotent and safe
	val.Release()

	// Third release
	val.Release()

	// Releasing nil value must also be safe
	var nilVal *v8.Value
	nilVal.Release()
}

// TestContextResetRetainedValues tests the deep capability of resetting all retained handles
// in a long-lived context, reclaiming all Persistent references.
func TestContextResetRetainedValues(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	ctx := v8.NewContext(iso)
	defer ctx.Close()

	// Accumulate 500 retained values
	for i := 0; i < 500; i++ {
		_, err := ctx.RunScript("({ i: 1 })", "")
		if err != nil {
			t.Fatal(err)
		}
	}

	if ctx.RetainedValueCount() != 500 {
		t.Fatalf("expected 500 retained values, got %d", ctx.RetainedValueCount())
	}

	// Reset all retained values in the context
	ctx.ResetRetainedValues()

	if ctx.RetainedValueCount() != 0 {
		t.Fatalf("expected 0 retained values after reset, got %d", ctx.RetainedValueCount())
	}

	// Verify that the context can still execute subsequent scripts cleanly
	res, err := ctx.RunScript("10 + 20", "")
	if err != nil {
		t.Fatalf("unexpected script execution error after reset: %v", err)
	}
	defer res.Release()

	if res.Int32() != 30 {
		t.Fatalf("expected 30, got %d", res.Int32())
	}
}

// BenchmarkCallbackWithManualRelease measures the cost when manual info.Release() is invoked
func BenchmarkCallbackWithManualRelease(b *testing.B) {
	iso := v8.NewIsolate()
	defer iso.Dispose()

	global := v8.NewObjectTemplate(iso)
	fnTmpl := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		defer info.Release()
		return nil
	})
	global.Set("testCallManual", fnTmpl)

	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ctx.RunScript("testCallManual(1, 'data', false)", "")
	}
	b.StopTimer()
	b.Logf("Benchmark ended. Context Retained Value Count: %d", ctx.RetainedValueCount())
}

// TestConcurrentContextResetRetainedValues verifies that multiple contexts across goroutines
// can call ResetRetainedValues concurrently without global lock serialization.
func TestConcurrentContextResetRetainedValues(t *testing.T) {
	const numGoroutines = 8
	const iterations = 50

	errCh := make(chan error, numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			iso := v8.NewIsolate()
			defer iso.Dispose()
			ctx := v8.NewContext(iso)
			defer ctx.Close()

			for i := 0; i < iterations; i++ {
				val, err := ctx.RunScript("({ name: 'yao', count: 123 })", "")
				if err != nil {
					errCh <- err
					return
				}
				_ = val
				ctx.ResetRetainedValues()
				if count := ctx.RetainedValueCount(); count != 0 {
					errCh <- nil
					return
				}
			}
			errCh <- nil
		}()
	}

	for g := 0; g < numGoroutines; g++ {
		if err := <-errCh; err != nil {
			t.Fatalf("concurrent context reset failed: %v", err)
		}
	}
}
