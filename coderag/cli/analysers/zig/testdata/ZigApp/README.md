# ZigApp

Small Zig project used by the codeRAG Zig analyser's smoke tests.
Exercises every entity type the analyser must recognise:

| File | Demonstrates |
|------|-------------|
| `src/main.zig` | `pub fn`, `pub const` struct + value, `pub fn` free helper, test block, cross-file calls into every other module |
| `src/order.zig` | `pub const` enum + struct, `pub fn`, error set, helper function, test |
| `src/inventory.zig` | `pub const` struct with methods, error set, `@import("std")` |
| `src/order_service.zig` | struct with methods, free function, cross-file calls |

Build with the Zig toolchain:

```bash
zig build
```

Scan with the Zig analyser:

```bash
zig_analyser.exe scan --dir . --output ./.lightrag
ls .lightrag/
```
