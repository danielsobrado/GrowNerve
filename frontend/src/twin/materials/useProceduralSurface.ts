import { ProceduralMaterial, type MaterialRecipe, type ProceduralMaterialBackend } from "@drusniel/ptl-runtime";
import { useThree } from "@react-three/fiber";
import { useEffect, type RefObject } from "react";
import type { Material, Mesh } from "three";

interface PreviousMeshMaterials {
  material: Material | Material[];
  customDepthMaterial?: Material;
  customDistanceMaterial?: Material;
}

const rendererBackend = (renderer: unknown): ProceduralMaterialBackend => {
  const candidate = renderer as { isWebGPURenderer?: boolean };
  return candidate.isWebGPURenderer ? "webgpu" : "webgl";
};

export function useProceduralSurface(meshRef: RefObject<Mesh | null>, recipe: MaterialRecipe): void {
  const renderer = useThree((state) => state.gl);

  useEffect(() => {
    const mesh = meshRef.current;
    if (!mesh) return;

    const previous: PreviousMeshMaterials = {
      material: mesh.material,
      customDepthMaterial: mesh.customDepthMaterial,
      customDistanceMaterial: mesh.customDistanceMaterial,
    };
    const procedural = new ProceduralMaterial(recipe, {
      backend: rendererBackend(renderer),
      coordinateSpace: recipe.coordinateSpace,
      textureFieldSource: "generated",
      generatedTextureFields: { resolution: 128 },
    });
    let active = true;

    void procedural.prepare().then(() => {
      if (!active || meshRef.current !== mesh) return;
      procedural.applyTo(mesh);
    }).catch((cause: unknown) => {
      console.error("GrowNerve PTL material preparation failed", cause);
    });

    return () => {
      active = false;
      if (meshRef.current === mesh && mesh.material === procedural.material) {
        mesh.material = previous.material;
        mesh.customDepthMaterial = previous.customDepthMaterial;
        mesh.customDistanceMaterial = previous.customDistanceMaterial;
      }
      procedural.dispose();
    };
  }, [meshRef, recipe, renderer]);
}
