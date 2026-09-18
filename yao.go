package v8go

import (
	"fmt"
	"unsafe"
)

// #include "v8go.h"
// #include <stdlib.h>
import "C"

// YaoNewIsolate creates a new V8 isolate. Only one thread may access
// a given isolate at a time, but different threads may access
// different isolates simultaneously.
// When an isolate is no longer used its resources should be freed
// by calling iso.Dispose().
// An *Isolate can be used as a v8go.ContextOption to create a new
// Context, rather than creating a new default Isolate.
func YaoNewIsolate() *Isolate {
	iso := &Isolate{
		ptr: C.YaoNewIsolate(),
		cbs: make(map[int]FunctionCallback),
	}
	iso.null = newValueNull(iso)
	iso.undefined = newValueUndefined(iso)
	return iso
}

// YaoNewIsolateFromGlobal creates a new V8 isolate from global.
func YaoNewIsolateFromGlobal() (*Isolate, error) {

	ptr := C.YaoNewIsolateFromGlobal()
	if ptr == nil {
		return nil, fmt.Errorf("YaoNewIsolateFromGlobal failed")
	}

	iso := &Isolate{
		ptr: ptr,
		cbs: make(map[int]FunctionCallback),
	}

	return iso, nil
}

// YaoDispose will dispose the Isolate VM; subsequent calls will panic.
func YaoDispose() {
	C.YaoDispose()
}

// Copy copies the current isolate.
func (iso *Isolate) Copy() (*Isolate, error) {
	if iso.ptr == nil {
		return nil, fmt.Errorf("invalid isolate")
	}
	new := &Isolate{
		ptr: C.YaoCopyIsolate(iso.ptr),
		cbs: make(map[int]FunctionCallback),
	}
	return new, nil
}

// Context returns the current context for this isolate.
// The internal context is managed by the Isolate and will be disposed automatically
// when the Isolate is disposed. Calling Close() on this context is safe and will be ignored.
func (iso *Isolate) Context() (*Context, error) {
	return iso.getOrCreateInternalContext()
}

// AsGlobal makes the isolate into a global object.
func (iso *Isolate) AsGlobal() {
	C.YaoIsolateAsGlobal(iso.ptr)
}

// YaoInit initializes V8 with the given heap size limit.
func YaoInit(heapSizeLimitMB uint) {
	v8once.Do(func() {
		cflags := C.CString("--no-freeze_flags_after_init")
		if heapSizeLimitMB > 0 {
			cflags = C.CString(fmt.Sprintf("--no-freeze_flags_after_init --max_old_space_size=%d", heapSizeLimitMB))
		}
		defer C.free(unsafe.Pointer(cflags))
		C.SetFlags(cflags)
		C.Init()
	})
}
