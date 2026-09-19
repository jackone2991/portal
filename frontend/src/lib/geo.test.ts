import { describe, expect, it } from "vitest";
import { tileXToLon } from "./geo";

describe("tileXToLon", () => {
  it("maps the tile row edge to edge", () => {
    expect(tileXToLon(0, 0)).toBe(-180);
    expect(tileXToLon(0.5, 0)).toBe(0);
    expect(tileXToLon(2, 2)).toBe(0);
  });

  it("wraps a pin past the antimeridian back onto the Earth", () => {
    // tile x beyond the row: the map panned past 180°E
    expect(tileXToLon(1.25, 0)).toBe(-90); // 270° → −90°
    expect(tileXToLon(1, 0)).toBe(-180); // exactly 180° reads as −180°
    expect(tileXToLon(-0.25, 0)).toBe(90); // panned past 180°W
  });
});
