# Upstream Issues for `onflow/cadence`

Issues in the Cadence AST `Doc()` and `Walk()` methods that affect external formatting tools.

---

## 1. `EntitlementDeclaration.Doc()` uses `HardLine` for access modifiers

`EntitlementDeclaration.Doc()` and `EntitlementMappingDeclaration.Doc()` use `prettier.HardLine{}` after the access modifier, forcing a line break:

```go
// entitlement_declaration.go:111
doc = append(doc, docOrEmpty(d.Access), prettier.HardLine{})
```

This produces:

```cadence
access(all)
entitlement NodeOperator
```

All other declaration types (`FunctionDeclaration`, `CompositeDeclaration`, `FieldDeclaration`, etc.) use `prettier.Line{}`, which keeps the access modifier on the same line when it fits:

```cadence
access(all) fun foo()
access(all) struct Bar {}
```

**Suggestion**: Change `prettier.HardLine{}` to `prettier.Line{}` in both `EntitlementDeclaration.Doc()` and `EntitlementMappingDeclaration.Doc()`.

---

## 2. `CastingExpression.Doc()` missing `Indent` on continuation

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

When the line breaks, `as!` appears at the same indentation as the expression, making it look like a separate statement:

```cadence
FlowToken.createEmptyVault(vaultType: Type<@FlowToken.Vault>())
as! @FlowToken.Vault
```

**Suggestion**: Wrap the continuation in `prettier.Indent{}`:

```go
prettier.Group{
    Doc: prettier.Concat{
        prettier.Group{Doc: doc},
        prettier.Indent{
            Doc: prettier.Concat{
                prettier.Line{},
                e.Operation.Doc(),
                prettier.Space,
                docOrEmpty(e.TypeAnnotation),
            },
        },
    },
}
```

---

## 3. Move operator `<-` spacing inconsistency

The move operator `<-` is formatted inconsistently across `Doc()` methods:

- `return <-expr` (no space after `<-`)
- `self.x <- expr` (space around `<-`)

This appears to stem from different handling of `TransferOperation` rendering in return statements vs assignment statements.

---

## 4. `InvocationExpression.Walk()` doesn't yield `Argument` wrappers

`InvocationExpression.Walk()` yields `arg.Expression` for each argument, but not the `Argument` struct itself:

```go
func (e *InvocationExpression) Walk(walkChild func(Element)) {
    walkChild(e.InvokedExpression)
    for _, typeArgument := range e.TypeArguments {
        walkChild(typeArgument)
    }
    for _, argument := range e.Arguments {
        walkChild(argument.Expression)  // only the Expression, not the Argument
    }
}
```

Since `Argument` has positional information (`LabelStartPos`, `LabelEndPos`, `TrailingSeparatorPos`), tools that map source positions to AST nodes (e.g., for comment attachment) cannot correctly associate comments that fall between argument labels and their expressions.

For comparison, `ParameterList.Walk()` yields individual `Parameter` elements.

**Suggestion**: Yield the `Argument` struct (or make `Argument` implement `Element` so it can be yielded by `Walk`).
