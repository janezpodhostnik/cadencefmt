# Upstream Issues for `onflow/cadence`

Issues in the Cadence AST `Doc()` methods that affect external formatting tools.
Items 1, 3, 4 from the original list are fixed by onflow/cadence#4485.

---

## 1. `CastingExpression.Doc()` missing `Indent` on continuation

`CastingExpression.Doc()` uses `prettier.Line{}` between the expression and the `as`/`as!`/`as?` operator without wrapping in `prettier.Indent{}`:

```go
prettier.Group{
    Doc: prettier.Concat{
        prettier.Group{Doc: doc},
        prettier.Line{},           // breaks to next line at same indent
        e.Operation.Doc(),
        prettier.Space,
        docOrEmpty(e.TypeAnnotation),
    },
}
```

When the line breaks, `as!` appears at the same indentation as the expression, making it look like a separate statement.

**Suggestion**: Wrap the continuation in `prettier.Indent{}`.

---

## 2. `BinaryExpression.Doc()` missing `Indent` on continuation

Same issue as `CastingExpression.Doc()` above. `BinaryExpression.Doc()` places the operator continuation (`&&`, `||`, `+`, etc.) at the same indent level as the left operand:

```go
return prettier.Group{
    Doc: prettier.Concat{
        prettier.Group{Doc: leftDoc},
        prettier.Line{},           // breaks without indent
        e.Operation.Doc(),
        prettier.Space,
        prettier.Group{Doc: rightDoc},
    },
}
```

**Suggestion**: Wrap `Line{}, op, Space, right` in `prettier.Indent{}`.

---

## 3. `FunctionDocument()` adds spurious space before `()` in anonymous functions

`FunctionDocument()` (used by `FunctionExpression.Doc()`) produces `fun (): Void { ... }` with a space between `fun` and `()`. This happens because the function uses `prettier.Line{}` after the access modifier position, and for anonymous functions with no access modifier, the `fun` keyword is followed by `Line{}` which becomes a space in flat mode.

**Example**: `fun(): Void { ... }` → `fun (): Void { ... }`

**Suggestion**: Skip the `Line{}` after `fun` when there is no access modifier.

---

## 4. `EntitlementMappingDeclaration.Doc()` missing `Group` wrapper

The `HardLine` → `Line` fix (onflow/cadence#4485) was applied to the access modifier, but `EntitlementMappingDeclaration.Doc()` was not wrapped in a `prettier.Group{}` (unlike `EntitlementDeclaration.Doc()` which was). The body's `HardLine` elements cause the `Line` after the access modifier to break unconditionally. The elements inside the mapping body are also not wrapped in `Indent`.
