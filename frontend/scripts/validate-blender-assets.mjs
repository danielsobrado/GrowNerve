import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
function parse(bytes) {
 assert.equal(bytes.readUInt32LE(0),0x46546c67);
 assert.equal(bytes.readUInt32LE(8),bytes.length);
 const length=bytes.readUInt32LE(12);
 return {doc:JSON.parse(bytes.subarray(20,20+length)),bin:bytes.subarray(28+length)};
}
for(const model of ['lettuce','tent','reservoir','light','fan','controller','pump']){
 const {doc,bin}=parse(await readFile(`public/models/blender/${model}.glb`));
 const {doc:source}=parse(await readFile(`../assets/blender/imports/${model}.glb`));
 const sourceKeys=new Map(source.nodes.filter(n=>n.extras?.geometryKey).map(n=>[n.extras.geometryKey,source.meshes[n.mesh]]));
 let triangles=0;
 for(const node of doc.nodes){
  const original=sourceKeys.get(node.extras.geometryKey);assert.ok(original,`${model}: unknown source mesh`);
  const p=doc.meshes[node.mesh].primitives[0];
  const count=doc.accessors[p.attributes.POSITION].count;
  for(const [semantic,idx] of Object.entries(p.attributes)){
   const a=doc.accessors[idx];assert.equal(a.count,count);
   const view=doc.bufferViews[a.bufferView];
   for(let offset=view.byteOffset;offset<view.byteOffset+view.byteLength;offset+=4){
    const value=bin.readFloatLE(offset);assert.ok(Number.isFinite(value),`${model}: nonfinite ${semantic}`);
    if(semantic==='_SOURCE_VERTEX') assert.ok(Number.isInteger(value)&&value>=0&&value<source.accessors[original.primitives[0].attributes.POSITION].count);
   }
  }
  const indexView=doc.bufferViews[doc.accessors[p.indices].bufferView];
  for(let offset=indexView.byteOffset;offset<indexView.byteOffset+indexView.byteLength;offset+=4)assert.ok(bin.readUInt32LE(offset)<count);
  triangles+=doc.accessors[p.indices].count/3;
 }
 assert.ok(triangles>0);
 console.log(`${model}: ${doc.meshes.length} valid refined meshes, ${triangles} triangles`);
}
