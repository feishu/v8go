package v8go_test

import (
	"fmt"
	"sync"
	"testing"

	v8 "rogchap.com/v8go"
)

func TestYaoVersionPrint(t *testing.T) {
	v := v8.Version()
	fmt.Println("Version", v)
}

func TestYaoInit(t *testing.T) {
	t.Parallel()
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	// stat := iso.GetHeapStatistics()
	// if stat.HeapSizeLimit > 1100*1024*1024 {
	// 	t.Errorf("HeapSizeLimit error %d", stat.HeapSizeLimit)
	// }
}

func TestYaoCopyIsolate(t *testing.T) {
	t.Parallel()
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	new, err := iso.Copy()
	if err != nil {
		t.Error(err)
	}
	defer new.Dispose()
}

func TestYaoIsolateContext(t *testing.T) {
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	_, err := iso.Context()
	if err != nil {
		t.Error(err)
	}
}

func TestYaoIsolateContextConcurrent(t *testing.T) {
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	const workers = 50
	var wg sync.WaitGroup
	contexts := make([]*v8.Context, workers)
	errors := make([]error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, err := iso.Context()
			contexts[idx] = ctx
			errors[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("worker %d failed: %v", i, err)
		}
	}

	firstCtx := contexts[0]
	if firstCtx == nil {
		t.Fatal("expected non-nil context")
	}
	for i := 1; i < workers; i++ {
		if contexts[i] != firstCtx {
			t.Errorf("worker %d got different context instance", i)
		}
	}
}

func TestYaoIsolateContextCallback(t *testing.T) {
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	ctx, err := iso.Context()
	if err != nil {
		t.Fatalf("failed to get internal context: %v", err)
	}

	called := false
	tmpl := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		called = true
		if info.Context() == nil {
			t.Error("expected non-nil callback context")
		}
		val, _ := v8.NewValue(iso, int32(42))
		return val
	})

	fn := tmpl.GetFunction(ctx)
	ctx.Global().Set("testFn", fn)

	res, err := ctx.RunScript("testFn()", "test.js")
	if err != nil {
		t.Fatalf("failed to run script: %v", err)
	}
	if !called {
		t.Fatal("expected callback to be called")
	}
	if !res.IsInt32() || res.Int32() != 42 {
		t.Fatalf("expected result 42, got %v", res)
	}
}

func TestYaoIsolateContextCloseProtection(t *testing.T) {
	v8.YaoInit(1024)
	defer v8.YaoDispose()

	iso := v8.YaoNewIsolate()
	defer iso.Dispose()

	ctx, err := iso.Context()
	if err != nil {
		t.Fatalf("failed to get internal context: %v", err)
	}

	// Calling Close() on internal context must be safely ignored
	ctx.Close()

	// Context should still work normally
	res, err := ctx.RunScript("100 + 200", "test.js")
	if err != nil {
		t.Fatalf("script failed after protected Close: %v", err)
	}
	if !res.IsInt32() || res.Int32() != 300 {
		t.Fatalf("expected 300, got %v", res)
	}
}

// func TestYaoNewIsolateFromGlobal(t *testing.T) {
// 	v8.YaoInit(1024)
// 	defer v8.YaoDispose()

// 	iso := v8.YaoNewIsolate()
// 	iso.AsGlobal()
// 	iso.Dispose()

// 	new, err := v8.YaoNewIsolateFromGlobal()
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	if new == nil {
// 		t.Errorf("new is nil")
// 	}
// 	defer new.Dispose()
// }
