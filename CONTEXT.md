# v8go Runtime Context

Go bindings for Google's V8 JavaScript engine, tailored for the Yao application engine.

## Language

**Isolate**:
A self-contained V8 virtual machine instance with its own isolated heap, GC, and single-threaded execution lock.
_Avoid_: VM, Engine, Runtime Instance

**Context**:
A sandboxed JavaScript execution environment with its own global object and built-in prototypes inside an Isolate.
_Avoid_: Scope, Realm, Sandbox

**Internal Context**:
An isolate-owned default context created alongside the Isolate, used for isolate-level primitive creation and internal script evaluation.
_Avoid_: Global Context, System Context, Root Context

**Context Reference**:
A unique integer identifier stored in V8 Context EmbedderData slot 1, enabling two-way lookup between V8 C++ callbacks and Go Context structures.
_Avoid_: Context ID, Ref ID, Session Key

**Value Codec**:
A binary serializer and deserializer module for bulk object and array transfer across the CGO boundary in a single operation.
_Avoid_: Data Converter, Object Marshaller, CGO Bridge
