# v8go - Project Rules

Reply in Chinese; use Conventional Commits with Chinese or English descriptions (e.g., `feat(codec): ...`, `fix(isolate): ...`).

## Overview

v8go provides high-performance Go bindings for the Google V8 JavaScript engine via CGO. This fork is heavily optimized and customized for the Yao application engine, incorporating zero-copy binary array buffer codecs, fast C++ JSON serialization, nanosecond-level context scope recycling, and V8 CodeCache preheating.

- **Ecosystem Role**: Embedded JavaScript/TypeScript engine binding powering `gou/runtime/v8` and `yao`.

## Tech Stack

- **Core Runtime**: Go 1.17+ (compiled with Go 1.24/1.25 toolchain)
- **C/C++ Standard**: C++17
- **V8 Engine**: Pre-built static V8 libraries in `deps/` (macOS arm64/x64, Linux amd64/arm64)
- **Build Requirements**: `CGO_ENABLED=1`, clang/gcc supporting C++17

## Common Commands & Fast Feedback Loop

```bash
# Environment requirement
export CGO_ENABLED=1

# Fast Feedback Inner Loop (<2s)
go test -v -run TestJSONStringifyBytes .    # Verify C++ fast JSON stringifier
go test -v -run TestZeroCopy .              # Verify zero-copy binary array buffer codec
go test -v -run TestContextRecycle .        # Verify context scope reuse & handle reset
go test -v -run Leak .                      # Verify memory leak safety
go test -bench=. -benchmem .                # Benchmark execution and CGO overhead
```

## Navigation & Key Entrypoints

| Subsystem / Feature | Primary Entry File / Directory |
| :--- | :--- |
| **C++ Bridge Implementation** | `v8go.cc`, `v8go.h` |
| **V8 Isolate & Heap Lifecycle** | `isolate.go` |
| **Context Scope Recycler (`ResetRetainedValues`)** | `context.go` |
| **Zero-Copy Binary ArrayBuffer & Codec** | `codec.go` |
| **C++ Native JSON Stringifier** | `json.go` |
| **Script Compilation & CodeCache** | `script_compiler.go` |
| **Custom Yao Extensions** | `yao.go` |
| **Precompiled V8 Headers & Libraries** | `deps/` |

## Directory Structure

```
v8go/
├── deps/              # Precompiled Google V8 static libraries and C++ headers (include/)
├── v8go.h             # C/C++ bridge header declarations for V8 APIs
├── v8go.cc            # C++ implementation bridging V8 C++ classes to CGO
├── isolate.go         # V8 Isolate lifecycle, heap statistics, CPU profiler & execution termination
├── context.go         # V8 Context management, scope recycling & retained-value reset
├── codec.go           # Zero-copy binary ArrayBuffer & Uint8Array codecs
├── json.go            # Native C++ JSON.stringify without Go string allocations
├── script_compiler.go # V8 ScriptCompiler, UnboundScript & CodeCache generation
├── template.go        # ObjectTemplate & FunctionTemplate base definitions
├── function.go        # Go-to-JavaScript callback bindings and function invocations
├── value.go           # V8 Value representations, type checks & primitive extractions
├── promise.go         # JavaScript Promise resolution, rejection & callback loops
├── yao.go             # Yao-specific engine extensions & helper exports
└── errors.go          # V8 execution errors, stack traces & JS exception parsing
```

## System Invariants (DO NOT BREAK)

1. **Isolate Thread Concurrency**: V8 Isolates are strictly single-threaded. Any access across goroutines requires either pooling isolates (via `gou`) or acquiring `v8::Locker`.
2. **NativeContext Count Invariant**: Reusing a context via `ctx.ResetRetainedValues()` MUST keep the number of native contexts warm at 1; never allow context or handle accumulation.
3. **CGO Memory Ownership**: Every CGO-allocated pointer (`C.ValuePtr`, `C.ContextPtr`) must be tracked with explicit `.Release()` / `.Close()` or finalizers.

## Footguns & Anti-Patterns (DO NOT)

- **DO NOT** omit `v8::HandleScope` in custom C++ functions (leads to silent isolate heap memory leaks).
- **DO NOT** pass Go pointers containing Go pointers across C boundaries (violates `cmd/cgo` pointer passing rules).
- **DO NOT** allocate intermediate Go strings when stringifying JS objects; always use `JSONStringifyBytes`.
- **DO NOT** call `isolate.TerminateExecution()` without handling the resulting execution error gracefully.
