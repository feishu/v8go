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

func TestJSONStringifyBigInt(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	// Case 1: Safe integer BigInt converts to JSON Number
	val1, err := ctx.RunScript(`({ userId: 12222222n, expireTime: 1792984236522n })`, "test_safe_bigint.js")
	if err != nil {
		t.Fatalf("unexpected script error: %v", err)
	}

	raw1, err := v8.JSONStringifyBytes(ctx, val1)
	if err != nil {
		t.Fatalf("unexpected stringify error on safe BigInt: %v", err)
	}
	expected1 := `{"userId":12222222,"expireTime":1792984236522}`
	if string(raw1) != expected1 {
		t.Errorf("expected %s, got %s", expected1, string(raw1))
	}

	// Case 2: Huge integer BigInt (> Number.MAX_SAFE_INTEGER) converts to JSON String without error or precision loss
	val2, err := ctx.RunScript(`({ hugeId: 9007199254740995n, negativeHuge: -9007199254740995n })`, "test_huge_bigint.js")
	if err != nil {
		t.Fatalf("unexpected script error: %v", err)
	}

	raw2, err := v8.JSONStringifyBytes(ctx, val2)
	if err != nil {
		t.Fatalf("unexpected stringify error on huge BigInt: %v", err)
	}
	expected2 := `{"hugeId":"9007199254740995","negativeHuge":"-9007199254740995"}`
	if string(raw2) != expected2 {
		t.Errorf("expected %s, got %s", expected2, string(raw2))
	}
}

func TestJSONStringifyView(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Isolate().Dispose()
	defer ctx.Close()

	if err := v8.JSONStringifyView(ctx, nil, func(view []byte) error { return nil }); err == nil {
		t.Error("expected error for nil val, got nil")
	}

	val, err := v8.JSONParseBytes(ctx, []byte(`{"title":"zero-copy","count":42}`))
	if err != nil {
		t.Fatal(err)
	}

	var captured string
	err = v8.JSONStringifyView(ctx, val, func(view []byte) error {
		captured = string(view)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `{"title":"zero-copy","count":42}`
	if captured != expected {
		t.Errorf("expected %s, got %s", expected, captured)
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
