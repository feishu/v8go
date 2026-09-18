// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go

// #include <stdlib.h>
// #include <stdint.h>
// #include <stddef.h>
// #include "v8go.h"
import "C"
import (
	"errors"
	"unsafe"
)

// SerializeValue serializes a JavaScript value using V8's native ValueSerializer.
// The serialized binary format matches the HTML structured clone algorithm.
func SerializeValue(ctx *Context, val Valuer) ([]byte, error) {
	if val == nil || val.value() == nil {
		return nil, errors.New("v8go: Value is required")
	}
	v := val.value()
	var ctxPtr C.ContextPtr
	if ctx != nil {
		ctxPtr = ctx.ptr
	} else if v.ctx != nil {
		ctxPtr = v.ctx.ptr
	}

	rtn := C.ValueSerialize(ctxPtr, v.ptr)
	if rtn.error.msg != nil {
		if rtn.data != nil {
			C.free(unsafe.Pointer(rtn.data))
		}
		return nil, newJSError(rtn.error)
	}

	if rtn.data == nil || rtn.length == 0 {
		if rtn.data != nil {
			C.free(unsafe.Pointer(rtn.data))
		}
		return []byte{}, nil
	}

	b := C.GoBytes(unsafe.Pointer(rtn.data), C.int(rtn.length))
	C.free(unsafe.Pointer(rtn.data))
	return b, nil
}

// Serialize serializes this Value using V8's native ValueSerializer.
func (v *Value) Serialize() ([]byte, error) {
	return SerializeValue(v.ctx, v)
}

// DeserializeValue deserializes binary data created by ValueSerializer into a JavaScript Value in the given Context.
func DeserializeValue(ctx *Context, data []byte) (*Value, error) {
	if ctx == nil || ctx.ptr == nil {
		return nil, errors.New("v8go: Context is required")
	}
	if len(data) == 0 {
		return nil, errors.New("v8go: Data is required")
	}

	dataPtr := (*C.uint8_t)(unsafe.Pointer(&data[0]))
	rtn := C.ContextDeserialize(ctx.ptr, dataPtr, C.size_t(len(data)))
	return valueResult(ctx, rtn)
}

// Deserialize deserializes binary data created by ValueSerializer into a JavaScript Value.
func (c *Context) Deserialize(data []byte) (*Value, error) {
	return DeserializeValue(c, data)
}

// NewUint8Array creates a new JavaScript Uint8Array in the context with direct bulk memory copy.
func NewUint8Array(ctx *Context, data []byte) (*Value, error) {
	if ctx == nil || ctx.ptr == nil {
		return nil, errors.New("v8go: Context is required")
	}

	var dataPtr *C.uint8_t
	if len(data) > 0 {
		dataPtr = (*C.uint8_t)(unsafe.Pointer(&data[0]))
	}

	rtn := C.ContextNewUint8Array(ctx.ptr, dataPtr, C.size_t(len(data)))
	return valueResult(ctx, rtn)
}

// NewUint8Array creates a new JavaScript Uint8Array in this Context with direct bulk memory copy.
func (c *Context) NewUint8Array(data []byte) (*Value, error) {
	return NewUint8Array(c, data)
}

// Uint8ArrayBytes extracts the underlying byte slice from a JavaScript Uint8Array.
// It returns an error if the value is not an instance of Uint8Array.
func Uint8ArrayBytes(val Valuer) ([]byte, error) {
	if val == nil || val.value() == nil {
		return nil, errors.New("v8go: Value is required")
	}
	v := val.value()

	rtn := C.ValueUint8ArrayBytes(v.ptr)
	if rtn.error.msg != nil {
		if rtn.data != nil {
			C.free(unsafe.Pointer(rtn.data))
		}
		return nil, newJSError(rtn.error)
	}

	if rtn.data == nil || rtn.length == 0 {
		if rtn.data != nil {
			C.free(unsafe.Pointer(rtn.data))
		}
		return []byte{}, nil
	}

	b := C.GoBytes(unsafe.Pointer(rtn.data), C.int(rtn.length))
	C.free(unsafe.Pointer(rtn.data))
	return b, nil
}

// Uint8Array extracts the underlying byte slice if this Value is a Uint8Array.
func (v *Value) Uint8Array() ([]byte, error) {
	return Uint8ArrayBytes(v)
}
