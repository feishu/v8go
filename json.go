// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include "v8go.h"
import "C"
import (
	"bytes"
	"errors"
	"unsafe"
)

// JSONParse tries to parse the string and returns it as *Value if successful.
// Any JS errors will be returned as `JSError`.
func JSONParse(ctx *Context, str string) (*Value, error) {
	if ctx == nil {
		return nil, errors.New("v8go: Context is required")
	}
	cstr := C.CString(str)
	defer C.free(unsafe.Pointer(cstr))

	rtn := C.JSONParse(ctx.ptr, cstr)
	return valueResult(ctx, rtn)
}

// JSONParseBytes tries to parse the byte slice as JSON and returns it as *Value if successful.
// Any JS errors will be returned as `JSError`.
func JSONParseBytes(ctx *Context, data []byte) (*Value, error) {
	if ctx == nil {
		return nil, errors.New("v8go: Context is required")
	}
	if len(data) == 0 {
		return JSONParse(ctx, "null")
	}
	dataPtr := (*C.char)(unsafe.Pointer(&data[0]))
	rtn := C.JSONParseBytes(ctx.ptr, dataPtr, C.int(len(data)))
	return valueResult(ctx, rtn)
}

// JSONStringify tries to stringify the JSON-serializable object value and returns it as string.
func JSONStringify(ctx *Context, val Valuer) (string, error) {
	if val == nil || val.value() == nil {
		return "", errors.New("v8go: Value is required")
	}
	// If a nil context is passed we'll use the context/isolate that created the value.
	var ctxPtr C.ContextPtr
	if ctx != nil {
		ctxPtr = ctx.ptr
	}

	str := C.JSONStringify(ctxPtr, val.value().ptr)
	defer C.free(unsafe.Pointer(str))
	return C.GoString(str), nil
}

// JSONStringifyView serializes the JSON-serializable object value and provides a zero-copy
// byte view to the callback function. The backing memory is freed immediately after fn returns.
// The view slice MUST NOT be used or retained outside of fn.
func JSONStringifyView(ctx *Context, val Valuer, fn func(view []byte) error) error {
	if val == nil || val.value() == nil {
		return errors.New("v8go: Value is required")
	}
	if fn == nil {
		return nil
	}

	var ctxPtr C.ContextPtr
	if ctx != nil {
		ctxPtr = ctx.ptr
	} else if val.value().ctx != nil {
		ctxPtr = val.value().ctx.ptr
	}

	rtn := C.JSONStringifyBytes(ctxPtr, val.value().ptr)
	if rtn.error.msg != nil {
		if rtn.data != nil {
			C.free(unsafe.Pointer(rtn.data))
		}
		return newJSError(rtn.error)
	}

	if rtn.data == nil || rtn.length == 0 {
		return fn([]byte{})
	}

	defer C.free(unsafe.Pointer(rtn.data))

	view := unsafe.Slice((*byte)(unsafe.Pointer(rtn.data)), int(rtn.length))
	return fn(view)
}

// JSONStringifyBytes serializes the JSON-serializable object value directly to byte slice.
func JSONStringifyBytes(ctx *Context, val Valuer) ([]byte, error) {
	var b []byte
	err := JSONStringifyView(ctx, val, func(view []byte) error {
		if len(view) == 0 {
			b = []byte{}
			return nil
		}
		b = bytes.Clone(view)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}
