import { describe, expect, it } from "vitest";
import { parseMaterialRecipe } from "@drusniel/ptl-runtime";
import { GROW_NERVE_PTL_RECIPES } from "./ptlRecipes";

describe("GrowNerve PTL material recipes", () => {
  it("keeps every shipped recipe portable and valid", () => {
    for (const recipe of Object.values(GROW_NERVE_PTL_RECIPES)) {
      const parsed = parseMaterialRecipe(recipe);
      expect(parsed.format).toBe("ptl-material");
      expect(parsed.layers.length).toBeGreaterThan(0);
    }
  });

  it("keeps scaled fixed surfaces world-aligned and small equipment object-aligned", () => {
    expect(GROW_NERVE_PTL_RECIPES["concrete-sealed"].coordinateSpace).toBe("world");
    expect(GROW_NERVE_PTL_RECIPES["hdpe-reservoir"].coordinateSpace).toBe("world");
    expect(GROW_NERVE_PTL_RECIPES["tent-fabric"].coordinateSpace).toBe("world");
    expect(GROW_NERVE_PTL_RECIPES["abs-dark"].coordinateSpace).toBe("object");
  });
});
