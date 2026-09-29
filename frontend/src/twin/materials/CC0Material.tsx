import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { DoubleSide, NoColorSpace, RepeatWrapping, SRGBColorSpace, TextureLoader, type Texture } from "three";
import type { ThreeElements } from "@react-three/fiber";

export type Surface = "metal" | "plastic" | "fabric" | "concrete" | "leaf";
export const TextureResolution = createContext<512 | 1024>(1024);
const loader = new TextureLoader();
const sources = new Map<string, Promise<Texture>>();
interface TextureSet { users: number; promise: Promise<Texture[]>; textures?: Texture[] }
const variants = new Map<string, TextureSet>();

function load(url: string): Promise<Texture> {
  let promise = sources.get(url);
  if (!promise) {
    promise = loader.loadAsync(url).catch((error: unknown) => { sources.delete(url); throw error; });
    sources.set(url, promise);
  }
  return promise;
}

type MaterialProps = Omit<ThreeElements["meshPhysicalMaterial"], "ref" | "args"> & {
  surface: Surface; repeat?: [number, number]; grain?: number; albedo?: boolean;
};

/** Cached local CC0 sources; each material owns its UV transforms and GPU textures. */
export function CC0Material({ surface, repeat = [1, 1], grain = 0.3, albedo = true, ...props }: MaterialProps) {
  const resolution = useContext(TextureResolution);
  const [maps, setMaps] = useState<{ key: string; textures: Texture[] }>();
  const [x, y] = repeat;
  const key = `${surface}/${resolution}/${x}/${y}`;
  const settings = useMemo(() => ({ surface, resolution, x, y }), [surface, resolution, x, y]);
  useEffect(() => {
    let active = true;
    let entry = variants.get(key);
    if (!entry) {
      const base = `${import.meta.env.BASE_URL}textures/cc0/${settings.surface}/`;
      const promise = Promise.all(["color", "normalgl", "roughness"].map((channel) => load(`${base}${channel}-${settings.resolution}.jpg`))).then((textures) => {
        const owned = textures.map((texture, index) => {
          const copy = texture.clone();
          copy.colorSpace = index === 0 ? SRGBColorSpace : NoColorSpace;
          copy.wrapS = copy.wrapT = RepeatWrapping;
          copy.repeat.set(settings.x, settings.y);
          copy.anisotropy = settings.resolution === 1024 ? 4 : 2;
          copy.needsUpdate = true;
          return copy;
        });
        entry!.textures = owned;
        if (entry!.users === 0) owned.forEach((texture) => texture.dispose());
        return owned;
      });
      entry = { users: 0, promise };
      variants.set(key, entry);
    }
    entry.users++;
    void entry.promise.then((textures) => { if (active) setMaps({ key, textures }); })
      .catch((error: unknown) => { if (active) console.error(`GrowNerve CC0 texture loading failed (${settings.surface})`, error); });
    return () => {
      active = false;
      if (--entry.users === 0) {
        entry.textures?.forEach((texture) => texture.dispose());
        variants.delete(key);
      }
    };
  }, [key, settings]);
  const textures = maps?.key === key ? maps.textures : undefined;
  return <meshPhysicalMaterial
    key={`${key}/${Boolean(textures)}`}
    map={albedo ? textures?.[0] : null} normalMap={textures?.[1]} roughnessMap={textures?.[2]}
    normalScale={[grain, grain]} roughness={0.7} side={surface === "leaf" ? DoubleSide : undefined}
    {...props}
  />;
}
