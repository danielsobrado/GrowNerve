import {
  createMaterialRecipe,
  type MaterialLayer,
  type MaterialRecipe,
  type PhysicalSettings,
  type RuntimeMaterialDefinition,
  type SynthesisSettings,
} from "@drusniel/ptl-runtime";

export type GrowNerveMaterialKey = "abs-dark" | "concrete-sealed" | "hdpe-reservoir" | "tent-fabric";

const SYNTHESIS: SynthesisSettings = {
  age: 0.08,
  weathering: 0.04,
  gravity: -1,
  macro: 0.7,
  meso: 0.8,
  micro: 0.9,
  variation: 0.22,
  stochasticTiling: 0,
};

const BASE_PHYSICAL: PhysicalSettings = {
  roughness: 0.62,
  metalness: 0,
  clearcoat: 0,
  clearcoatRoughness: 0.3,
  specularIntensity: 0.48,
  ior: 1.44,
  sheen: 0,
  sheenRoughness: 0.72,
  sheenColor: "#ffffff",
  transmission: 0,
  thickness: 0,
  attenuationDistance: 2,
  attenuationColor: "#ffffff",
};

const layer = (id: string, name: string, kind: MaterialLayer["kind"], overrides: Partial<MaterialLayer> = {}): MaterialLayer => ({
  id,
  name,
  kind,
  enabled: true,
  blendMode: "normal",
  channel: kind === "wet-film" ? "clearcoat" : kind === "sss" ? "sss" : kind === "vessels" ? "color" : "surface",
  opacity: 1,
  scale: 3,
  strength: 1,
  seed: 1,
  colorA: "#545862",
  colorB: "#d8dce6",
  roughness: 0,
  displacement: 0,
  groupId: null,
  maskSourceLayerId: null,
  structureSourceLayerId: null,
  maskInvert: false,
  maskStrength: 1,
  maskMode: "coverage",
  maskThreshold: 0.5,
  maskSoftness: 0.15,
  maskBreakup: 0,
  ...overrides,
});

const recipe = (
  seed: number,
  physical: Partial<PhysicalSettings>,
  layers: MaterialLayer[],
  synthesis: Partial<SynthesisSettings> = {},
  coordinateSpace: "object" | "world" = "object",
): MaterialRecipe => {
  const definition: RuntimeMaterialDefinition = {
    physical: { ...BASE_PHYSICAL, ...physical },
    synthesis: { ...SYNTHESIS, ...synthesis },
    groups: [],
    layers,
    surfaceGraph: null,
  };
  return createMaterialRecipe(definition, seed, coordinateSpace);
};

export const GROW_NERVE_PTL_RECIPES: Record<GrowNerveMaterialKey, MaterialRecipe> = {
  "hdpe-reservoir": recipe(18472, {
    roughness: 0.68,
    clearcoat: 0.04,
    clearcoatRoughness: 0.52,
    specularIntensity: 0.42,
  }, [
    layer("hdpe-base", "Moulded HDPE", "base", { colorA: "#3f4c45", colorB: "#59665e", roughness: 0.12 }),
    layer("hdpe-grain", "Fine moulding grain", "fbm", {
      blendMode: "overlay",
      opacity: 0.24,
      scale: 11,
      strength: 0.55,
      seed: 29,
      colorA: "#344039",
      colorB: "#718078",
      roughness: 0.1,
      displacement: 0.004,
    }),
  ]),
  "abs-dark": recipe(21931, {
    roughness: 0.56,
    clearcoat: 0.05,
    clearcoatRoughness: 0.42,
    specularIntensity: 0.52,
  }, [
    layer("abs-base", "Dark ABS", "base", { colorA: "#171d19", colorB: "#2a332d", roughness: 0.08 }),
    layer("abs-speckle", "ABS micro texture", "spots", {
      blendMode: "overlay",
      opacity: 0.16,
      scale: 14,
      strength: 0.42,
      seed: 41,
      colorA: "#111713",
      colorB: "#414b45",
      roughness: 0.12,
      displacement: 0.002,
    }),
  ]),
  "concrete-sealed": recipe(90817, {
    roughness: 0.82,
    clearcoat: 0.02,
    clearcoatRoughness: 0.78,
    specularIntensity: 0.3,
  }, [
    layer("concrete-base", "Sealed concrete", "base", { colorA: "#242b27", colorB: "#343d37", roughness: 0.2 }),
    layer("concrete-cloud", "Aggregate variation", "fbm", {
      blendMode: "overlay",
      opacity: 0.34,
      scale: 5.5,
      strength: 0.62,
      seed: 18,
      colorA: "#1f2622",
      colorB: "#4a554e",
      roughness: 0.13,
      displacement: 0.012,
    }),
    layer("concrete-grain", "Fine grain", "spots", {
      blendMode: "multiply",
      opacity: 0.12,
      scale: 16,
      strength: 0.38,
      seed: 63,
      colorA: "#1b211e",
      colorB: "#59645d",
      roughness: 0.08,
      displacement: 0.003,
    }),
  ], { variation: 0.28 }, "world"),
  "tent-fabric": recipe(71253, {
    roughness: 0.88,
    sheen: 0.12,
    sheenRoughness: 0.82,
    sheenColor: "#738078",
    specularIntensity: 0.24,
  }, [
    layer("fabric-base", "Grow tent fabric", "base", { colorA: "#0e1310", colorB: "#1b231e", roughness: 0.24 }),
    layer("fabric-weave", "Fabric weave", "ridges", {
      blendMode: "overlay",
      opacity: 0.18,
      scale: 18,
      strength: 0.48,
      seed: 52,
      colorA: "#080b09",
      colorB: "#354039",
      roughness: 0.11,
      displacement: 0.003,
    }),
  ], { micro: 1.25, variation: 0.14 }),
};
