# Upstream Issues for `onflow/cadence`

All previously documented issues have been fixed by onflow/cadence#4485.

- `CastingExpression.Doc()` — `Indent{}` added for continuation (commit 048f0af)
- `BinaryExpression.Doc()` — `Indent{}` added for continuation (commit 048f0af)
- `FunctionDocument()` — no spurious space before `()` in anonymous functions (commit 048f0af)
- `EntitlementMappingDeclaration.Doc()` — `Group{}` wrapper added (commit 048f0af)
- `EntitlementDeclaration.Doc()` — `HardLine` → `Line` + `Group{}` (commit e93ac01)
- Move operator spacing — `Space` added after `OperationMove` (commit e93ac01)
- `Argument` as walkable `Element` — `InvocationExpression.Walk()` yields `*ast.Argument` (commit e93ac01)

**Note**: `renderEntitlementMapping` is still needed in the formatter because the upstream `Group{}` fix handles access modifier positioning but doesn't indent the mapping body elements.
