// Copyright 2021 Roger Chapman and the v8go contributors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package v8go_test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	v8 "rogchap.com/v8go"
)

func TestCodecUint8ArrayRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := v8.NewContext()
	defer ctx.Close()

	// 1. Zero length slice
	emptyVal, err := ctx.NewUint8Array([]byte{})
	if err != nil {
		t.Fatalf("unexpected error creating empty Uint8Array: %v", err)
	}
	if !emptyVal.IsUint8Array() {
		t.Errorf("expected IsUint8Array() to be true")
	}
	emptyBytes, err := emptyVal.Uint8Array()
	if err != nil {
		t.Fatalf("unexpected error reading empty Uint8Array: %v", err)
	}
	if len(emptyBytes) != 0 {
		t.Errorf("expected 0 length, got %d", len(emptyBytes))
	}

	// 2. Small payload round-trip
	payload := []byte("Hello, V8 Direct Binary Codec!")
	val, err := ctx.NewUint8Array(payload)
	if err != nil {
		t.Fatalf("unexpected error creating Uint8Array: %v", err)
	}
	if !val.IsUint8Array() {
		t.Errorf("expected IsUint8Array() to be true")
	}
	out, err := val.Uint8Array()
	if err != nil {
		t.Fatalf("unexpected error reading Uint8Array: %v", err)
	}
	if !bytes.Equal(payload, out) {
		t.Errorf("binary content mismatch: expected %q, got %q", payload, out)
	}

	// 3. Mutation in JavaScript reflects in Go
	ctx.Global().Set("arr", val)
	_, err = ctx.RunScript("arr[0] = 65; arr[1] = 66;", "mutate.js")
	if err != nil {
		t.Fatalf("unexpected script execution error: %v", err)
	}
	mutated, err := val.Uint8Array()
	if err != nil {
		t.Fatalf("unexpected error reading mutated Uint8Array: %v", err)
	}
	if len(mutated) < 2 || mutated[0] != 65 || mutated[1] != 66 {
		t.Errorf("JavaScript modifications not reflected: %v", mutated[:2])
	}

	// 4. Large payload (1MB) round-trip
	large := make([]byte, 1024*1024)
	for i := range large {
		large[i] = byte(i % 256)
	}
	largeVal, err := ctx.NewUint8Array(large)
	if err != nil {
		t.Fatalf("unexpected error creating 1MB Uint8Array: %v", err)
	}
	largeOut, err := largeVal.Uint8Array()
	if err != nil {
		t.Fatalf("unexpected error reading 1MB Uint8Array: %v", err)
	}
	if !bytes.Equal(large, largeOut) {
		t.Errorf("1MB binary data content mismatch")
	}

	// 5. Type safety check
	strVal, _ := ctx.RunScript("'not an array'", "")
	if _, err := strVal.Uint8Array(); err == nil {
		t.Errorf("expected error when calling Uint8Array on non-Uint8Array value")
	}
}

func TestCodecStructuredClone(t *testing.T) {
	t.Parallel()

	ctx1 := v8.NewContext()
	defer ctx1.Close()

	// 1. Serialize complex object in ctx1
	val, err := ctx1.RunScript(`({
		id: 1001,
		name: "structured-clone-test",
		tags: ["yao", "v8", "bridge"],
		active: true,
		meta: { version: 2, score: 98.5 }
	})`, "clone.js")
	if err != nil {
		t.Fatalf("unexpected script error: %v", err)
	}

	serialized, err := val.Serialize()
	if err != nil {
		t.Fatalf("unexpected serialize error: %v", err)
	}
	if len(serialized) == 0 {
		t.Fatalf("expected non-empty serialized binary output")
	}

	// 2. Deserialize in separate context ctx2
	ctx2 := v8.NewContext()
	defer ctx2.Close()

	deserialized, err := ctx2.Deserialize(serialized)
	if err != nil {
		t.Fatalf("unexpected deserialize error: %v", err)
	}
	if !deserialized.IsObject() {
		t.Fatalf("expected deserialized value to be an Object")
	}

	ctx2.Global().Set("cloned", deserialized)
	checkVal, err := ctx2.RunScript(`
		cloned.id === 1001 &&
		cloned.name === "structured-clone-test" &&
		cloned.tags.length === 3 &&
		cloned.tags[1] === "v8" &&
		cloned.active === true &&
		cloned.meta.version === 2 &&
		cloned.meta.score === 98.5
	`, "check.js")
	if err != nil {
		t.Fatalf("unexpected check script error: %v", err)
	}
	if !checkVal.Boolean() {
		t.Errorf("deserialized object property verification failed")
	}

	// 3. Corrupt byte slice error handling
	corrupt := []byte{0xFF, 0xFE, 0x00, 0x12, 0x34}
	if _, err := ctx2.Deserialize(corrupt); err == nil {
		t.Errorf("expected error when deserializing corrupt bytes")
	}

	// 4. Empty slice error handling
	if _, err := ctx2.Deserialize(nil); err == nil {
		t.Errorf("expected error when deserializing nil slice")
	}
}

func TestCodecConcurrent(t *testing.T) {
	t.Parallel()

	const workers = 30
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			ctx := v8.NewContext()
			defer ctx.Close()

			// Test Uint8Array direct memory
			data := []byte(fmt.Sprintf("worker-payload-%d", workerID))
			arrVal, err := ctx.NewUint8Array(data)
			if err != nil {
				t.Errorf("worker %d failed to create Uint8Array: %v", workerID, err)
				return
			}
			readData, err := arrVal.Uint8Array()
			if err != nil {
				t.Errorf("worker %d failed to read Uint8Array: %v", workerID, err)
				return
			}
			if !bytes.Equal(data, readData) {
				t.Errorf("worker %d mismatch in Uint8Array", workerID)
				return
			}

			// Test ValueSerializer / ValueDeserializer
			script := fmt.Sprintf("({ worker: %d, ok: true })", workerID)
			objVal, err := ctx.RunScript(script, "")
			if err != nil {
				t.Errorf("worker %d script error: %v", workerID, err)
				return
			}
			bytesData, err := objVal.Serialize()
			if err != nil {
				t.Errorf("worker %d serialize error: %v", workerID, err)
				return
			}
			clonedVal, err := ctx.Deserialize(bytesData)
			if err != nil {
				t.Errorf("worker %d deserialize error: %v", workerID, err)
				return
			}
			if !clonedVal.IsObject() {
				t.Errorf("worker %d expected object", workerID)
			}
		}()
	}

	wg.Wait()
}

// --- Benchmarks ---

func BenchmarkUint8ArrayDirect_64KB(b *testing.B) {
	ctx := v8.NewContext()
	defer ctx.Close()

	payload := make([]byte, 64*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(payload)))
	for i := 0; i < b.N; i++ {
		val, err := ctx.NewUint8Array(payload)
		if err != nil {
			b.Fatal(err)
		}
		_, err = val.Uint8Array()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUint8ArrayLegacy_64KB(b *testing.B) {
	ctx := v8.NewContext()
	defer ctx.Close()

	payload := make([]byte, 64*1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(payload)))
	for i := 0; i < b.N; i++ {
		// Legacy SetIdx approach simulating gou/runtime/v8/bridge
		v, err := ctx.RunScript(fmt.Sprintf("new Uint8Array(%d)", len(payload)), "")
		if err != nil {
			b.Fatal(err)
		}
		obj, err := v.AsObject()
		if err != nil {
			b.Fatal(err)
		}
		for j := 0; j < len(payload); j++ {
			obj.SetIdx(uint32(j), int32(payload[j]))
		}

		// Legacy GetIdx approach
		out := make([]byte, len(payload))
		for j := 0; j < len(payload); j++ {
			elem, _ := obj.GetIdx(uint32(j))
			out[j] = byte(elem.Int32())
		}
	}
}

func BenchmarkStructuredCloneVsJSON_Serialize(b *testing.B) {
	ctx := v8.NewContext()
	defer ctx.Close()

	val, err := ctx.RunScript(`({
		id: 1001,
		title: "performance test object",
		items: Array.from({length: 100}, (_, i) => ({ id: i, active: i % 2 === 0 })),
		nested: { a: 1, b: "two", c: [1, 2, 3] }
	})`, "")
	if err != nil {
		b.Fatal(err)
	}

	b.Run("StructuredClone", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := val.Serialize()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("JSONStringify", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := v8.JSONStringify(ctx, val)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
