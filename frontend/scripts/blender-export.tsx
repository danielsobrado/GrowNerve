import React, { useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { Canvas, useThree } from '@react-three/fiber';
import { Group, Mesh, InstancedMesh, Matrix4, MeshStandardMaterial } from 'three';
import { GLTFExporter } from 'three/addons/exporters/GLTFExporter.js';
import { GrowTent, LettucePlant, DwcReservoir, GrowLight, CirculationFan, Controller, AirPump } from '../src/twin/models/GrowModels';
import { geometryKey } from '../src/twin/models/geometryKey';
const quality = {name:'desktop',geometryScale:1,shadows:true} as any;
function Exporter() {
 const scene=useThree(s=>s.scene);
 useEffect(()=>{
  (window as any).exportModel=async(name:string)=>{
   const source=scene.getObjectByName(name)!;
   source.updateWorldMatrix(true,true);
   const out=new Group(); out.name=name;
   const materials=new Map();
   source.traverse(o=>{
    if(!(o instanceof Mesh))return;
    const make=(matrix:Matrix4)=>{
     const original=o.material as MeshStandardMaterial;
     let material=materials.get(original);
     if(!material){ material=original.clone(); materials.set(original,material); }
     material.name=original.map?.image?.src?.match(/cc0\/(\w+)\//)?.[1]??'plain';
     const mesh=new Mesh(o.geometry,material);
     mesh.name=geometryKey(o.geometry);
     mesh.userData.geometryKey=mesh.name;
     mesh.applyMatrix4(matrix);out.add(mesh);
    };
    if(o instanceof InstancedMesh)for(let i=0;i<o.count;i++){
     const m=new Matrix4();o.getMatrixAt(i,m);make(o.matrixWorld.clone().multiply(m));
    }else make(o.matrixWorld);
   });
   console.log('Export geometry',name,out.children.length);
   const buffer=await new GLTFExporter().parseAsync(out,{binary:true,onlyVisible:false,maxTextureSize:1024});
   const url=URL.createObjectURL(new Blob([buffer as ArrayBuffer]));
   const a=document.createElement('a');a.href=url;a.download=`${name}.glb`;a.click();
   setTimeout(()=>URL.revokeObjectURL(url),10000);
  };
 },[scene]);
 return <>
  <group name="lettuce"><LettucePlant quality={quality} occupied attention={false} seed={0}/></group>
  <group name="tent"><GrowTent quality={quality}/></group>
  <group name="reservoir"><DwcReservoir quality={quality} level={60}/></group>
  <group name="light"><GrowLight quality={quality} running={false}/></group>
  <group name="fan"><CirculationFan quality={quality} running={false}/></group>
  <group name="controller"><Controller online/></group>
  <group name="pump"><AirPump running/></group>
 </>;
}
createRoot(document.getElementById('root')!).render(<Canvas frameloop="demand"><Exporter/></Canvas>);
