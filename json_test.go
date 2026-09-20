// Copyright 2021 Roger Chapman and the v8 contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"fmt"
	"testing"

	v8 "rogchap.com/v8go"
)

func TestJSONParse(t *testing.T) {
	t.Parallel()

	if _, err := v8.JSONParse(nil, "{}"); err == nil {
		t.Error("expected error but got <nil>")
	}
	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()
	_, err := v8.JSONParse(ctx, "{")
	if err == nil {
		t.Error("expected error but got <nil>")
		return
	}

	if _, ok := err.(*v8.JSError); !ok {
		t.Errorf("expected error to be of type JSError, got: %T", err)
	}
}

func TestJSONParseBytes(t *testing.T) {
	t.Parallel()

	if _, err := v8.JSONParseBytes(nil, []byte("{}")); err == nil {
		t.Error("expected error but got <nil>")
	}
	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	val, err := v8.JSONParseBytes(ctx, []byte(`{"hello": "world", "num": 123}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil || !val.IsObject() {
		t.Errorf("expected object, got: %v", val)
	}
}

func TestJSONStringify(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()
	if _, err := v8.JSONStringify(ctx, nil); err == nil {
		t.Error("expected error but got <nil>")
	}
}

func TestJSONStringifyBytes(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	if _, err := v8.JSONStringifyBytes(ctx, nil); err == nil {
		t.Error("expected error but got <nil>")
	}

	val, err := v8.JSONParseBytes(ctx, []byte(`{"a":1,"b":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}

	raw, err := v8.JSONStringifyBytes(ctx, val)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"a":1,"b":"hello"}`
	if string(raw) != expected {
		t.Errorf("expected %s, got %s", expected, string(raw))
	}
}

func ExampleJSONParse() {
	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()
	val, _ := v8.JSONParse(ctx, `{"foo": "bar"}`)
	fmt.Println(val)
	// Output:
	// [object Object]
}

func ExampleJSONStringify() {
	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()
	val, _ := v8.JSONParse(ctx, `{
		"a": 1,
		"b": "foo"
	}`)
	jsonStr, _ := v8.JSONStringify(ctx, val)
	fmt.Println(jsonStr)
	// Output:
	// {"a":1,"b":"foo"}
}
