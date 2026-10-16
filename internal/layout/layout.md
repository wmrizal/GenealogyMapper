# Layout

Implemented in `internal/layout/layout.go`. `compute` runs in three phases: assign generation levels, order each level, place nodes.

## 1. Levels (rows)

- `ComputeFull`: each person's level is the longest parent chain above them, so roots are on level 0.
- `Compute` (selected person): the selected person is level 0, ancestors are negative, descendants are positive, and siblings share level 0.
- In selected mode, everyone else (co-parents, cousins, in-laws) is placed relative to people already placed:
  - If they have a placed child, they go one level above that child (the shallowest child if there are several).
  - Otherwise, if they have a placed parent, they go one level below that parent (the deepest parent if there are several).
  - This repeats until nobody is left. Anyone still unplaced falls back to level 0.

Example with Bob Clark selected: Jane Lopez is Frank's other parent, so she lands on Dave's row, not Bob's.

## 2. Ordering within a level

`orderLevels` reduces crossing parent-child lines:

1. Seed: sort by name, then ID, so the result is deterministic.
2. Barycenter sweeps: up to 24 alternating down and up passes. Each person moves to the average position of their relatives in the neighboring level. Anyone with no such relatives keeps their position.
3. Transpose: after each pass, swap adjacent people in a row whenever that lowers the crossing count. Skipped above 300 relationships.
4. Keep the best ordering seen. It stops early at zero crossings.

Crossings are counted only between edges that connect the same two levels. Edges that skip a level are approximated.

## 3. Placement

- Y = `level * (NodeHeight + VerticalGap)`.
- X = `(index - (n-1)/2) * (NodeWidth + HorizontalGap)`, so every row is centered on the same axis.

## Tests

`internal/layout/layout_test.go` covers level placement, siblings, co-parent rows, zero crossings, and determinism.
