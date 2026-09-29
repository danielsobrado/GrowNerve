"""Run inside Blender through MCP, one source model per invocation.

Keeps source transforms and CC0 materials in editable scenes. The small GLBs
contain only revised local geometry; the app retains animation and live materials.
"""
import bpy, bmesh, math, json, struct
from pathlib import Path
from mathutils import Vector
from mathutils.kdtree import KDTree

ROOT = Path('F:/Development/GrowNerve')
DEST = ROOT / 'frontend/public/models/blender'
DEST.mkdir(parents=True, exist_ok=True)

def write_geometry_glb(name, objects):
    blob = bytearray()
    doc = {'asset': {'version': '2.0', 'generator': 'GrowNerve Blender geometry round trip'},
           'buffers': [], 'bufferViews': [], 'accessors': [], 'meshes': [], 'nodes': [],
           'scenes': [{'nodes': []}], 'scene': 0}
    def accessor(values, components, integer=False):
        while len(blob) % 4: blob.append(0)
        offset = len(blob)
        blob.extend(struct.pack('<' + ('I' if integer else 'f') * len(values), *values))
        view = len(doc['bufferViews'])
        doc['bufferViews'].append({'buffer': 0, 'byteOffset': offset, 'byteLength': len(blob)-offset})
        item = {'bufferView': view, 'componentType': 5125 if integer else 5126,
                'count': len(values)//components, 'type': {1:'SCALAR',2:'VEC2',3:'VEC3',4:'VEC4'}[components]}
        if components == 3:
            item['min'] = [min(values[i::3]) for i in range(3)]
            item['max'] = [max(values[i::3]) for i in range(3)]
        doc['accessors'].append(item)
        return len(doc['accessors'])-1
    stats=[]
    for key, obj in objects.items():
        evaluated = obj.evaluated_get(bpy.context.evaluated_depsgraph_get())
        mesh = evaluated.to_mesh(preserve_all_data_layers=True, depsgraph=bpy.context.evaluated_depsgraph_get())
        mesh.calc_loop_triangles()
        uv = mesh.uv_layers.active
        color = mesh.color_attributes.active_color
        vertices, normals, uvs, colors, indices, lookup = [], [], [], [], [], {}
        for tri in mesh.loop_triangles:
            for li in tri.loops:
                vi = mesh.loops[li].vertex_index
                co = mesh.vertices[vi].co
                normal = mesh.corner_normals[li].vector
                tex = tuple(uv.data[li].uv) if uv else (0,0)
                rgba = tuple(color.data[li if color.domain == 'CORNER' else vi].color) if color else ()
                signature = (vi, *[round(v,6) for v in normal], *tex, *rgba)
                if signature not in lookup:
                    lookup[signature] = len(vertices)//3
                    vertices.extend((co.x,co.z,-co.y))
                    normals.extend((normal.x,normal.z,-normal.y))
                    uvs.extend((tex[0],1-tex[1]))
                    if rgba: colors.extend(rgba)
                indices.append(lookup[signature])
        attributes={'POSITION':accessor(vertices,3),'NORMAL':accessor(normals,3),'TEXCOORD_0':accessor(uvs,2)}
        if colors: attributes['COLOR_0']=accessor(colors,4)
        idx = len(doc['meshes'])
        doc['meshes'].append({'name':key,'primitives':[{'attributes':attributes,'indices':accessor(indices,1,True)}]})
        doc['nodes'].append({'name':key,'mesh':idx,'extras':{'geometryKey':key}})
        doc['scenes'][0]['nodes'].append(idx)
        stats.append({'key':key,'vertices':len(vertices)//3,'triangles':len(indices)//3})
        evaluated.to_mesh_clear()
    doc['buffers']=[{'byteLength':len(blob)}]
    header=json.dumps(doc,separators=(',',':')).encode()
    header += b' ' * ((-len(header))%4)
    blob += b'\0' * ((-len(blob))%4)
    result=struct.pack('<III',0x46546c67,2,28+len(header)+len(blob))+struct.pack('<II',len(header),0x4e4f534a)+header+struct.pack('<II',len(blob),0x004e4942)+blob
    (DEST/f'{name}.glb').write_bytes(result)
    return stats

def refine(name):
    scene=bpy.data.scenes.new(f'GrowNerve - {name} refined')
    bpy.context.window.scene=scene
    bpy.ops.import_scene.gltf(filepath=str(ROOT/'assets/blender/imports'/f'{name}.glb'))
    changed={}
    unique={}
    for obj in scene.objects:
        if obj.type=='MESH' and obj.get('geometryKey'):
            unique.setdefault(obj['geometryKey'],[]).append(obj)
    for key, instances in unique.items():
        obj=instances[0]
        mesh=obj.data
        count=len(mesh.vertices)
        # All copies use one refined mesh and retain their independent transforms.
        for other in instances:
            materials=[slot.material for slot in other.material_slots]
            other.data=mesh
            for i,mat in enumerate(materials):
                if i<len(other.material_slots):
                    other.material_slots[i].link='OBJECT'
                    other.material_slots[i].material=mat
        foliage=name=='lettuce' and count>20000
        blade=name=='fan' and count>500 and obj.active_material and obj.active_material.name.startswith('plastic') and max(v.co.x for v in mesh.vertices)>0.4 and min(v.co.x for v in mesh.vertices)>0
        lining=name=='tent' and count>500 and obj.active_material and obj.active_material.name.startswith('metal')
        if foliage:
            # glTF import reorders vertices. Recover the authored leaf grid by position.
            raw=(ROOT/'assets/blender/imports'/f'{name}.glb').read_bytes()
            jslen=struct.unpack_from('<I',raw,12)[0]
            doc=json.loads(raw[20:20+jslen])
            acc=next(a for a in doc['accessors'] if a['type']=='VEC3' and a['count']==29600 and 'min' in a)
            view=doc['bufferViews'][acc['bufferView']]
            start=28+jslen+view.get('byteOffset',0)+acc.get('byteOffset',0)
            tree=KDTree(acc['count'])
            for j in range(acc['count']):
                x,y,z=struct.unpack_from('<fff',raw,start+j*view.get('byteStride',12))
                tree.insert((x,-z,y),j)
            tree.balance()
            for v in mesh.vertices:
                _,source_index,_=tree.find(v.co)
                leaf=source_index//925; row=(source_index%925)//25; column=source_index%25
                t=row/36; u=column/24*2-1; edge=abs(u); p=leaf/31
                angle=leaf*2.399963
                x,z=v.co.x,-v.co.y
                lx=x*math.cos(angle)-z*math.sin(angle)
                lz=x*math.sin(angle)+z*math.cos(angle)
                # Multi-scale marginal frills, raised midrib and asymmetric blade curl.
                frill=(math.sin(t*34+leaf*1.3+u*3)*0.018+math.sin(t*58-leaf)*0.005)*edge**2.4*math.sin(math.pi*t)
                rib=0.014*math.exp(-u*u*90)*math.sin(math.pi*t)
                secondary=0.005*math.cos((t-edge*.2)*math.pi*18)*math.sin(math.pi*t)*(1-edge)
                v.co.z+=frill+rib+secondary+0.028*u*math.sin(t*math.pi)*math.sin(leaf*1.9)-p*0.16*math.sin(t*math.pi*.7)
                lx*=1.08+0.1*math.sin(leaf*3.7)*t+0.035*math.sin(t*31+leaf)*edge**3
                lz*=1+0.055*math.sin(leaf*2.1)
                v.co.x=lx*math.cos(angle)+lz*math.sin(angle)
                v.co.y=-(lz*math.cos(angle)-lx*math.sin(angle))
            for poly in mesh.polygons: poly.use_smooth=True
            obj['refinement']='Asymmetric leaves, scalloped margins, raised midrib and secondary ribs'
        elif lining:
            for v in mesh.vertices:
                x,y,z=v.co
                v.co.y+=0.0035*math.sin(x*57+z*17)*math.sin(z*23)+0.002*math.sin(z*91+x*13)
            for poly in mesh.polygons: poly.use_smooth=True
            obj['refinement']='Fine cross folds on reflective lining'
        elif blade:
            for v in mesh.vertices:
                v.co.y+=0.065*(v.co.x-0.08)*(v.co.z/0.45)
            for poly in mesh.polygons: poly.use_smooth=True
            obj['refinement']='Progressive pitch on swept blades'
        elif count<=300 and len(mesh.polygons)>5:
            bm=bmesh.new();bm.from_mesh(mesh)
            bmesh.ops.remove_doubles(bm,verts=list(bm.verts),dist=0.000001)
            edges=[e for e in bm.edges if len(e.link_faces)==2 and e.calc_face_angle(0)>0.55]
            if not edges: bm.free();continue
            dimensions=[max(v.co[i] for v in bm.verts)-min(v.co[i] for v in bm.verts) for i in range(3)]
            width=min(dimensions)*0.055
            if width<0.00001: bm.free();continue
            bmesh.ops.bevel(bm,geom=edges,offset=width,segments=3,affect='EDGES',clamp_overlap=True)
            bm.to_mesh(mesh);bm.free()
            obj['refinement']='Three-segment edge bevels on manufactured parts'
        else:
            continue
        mesh.update()
        if foliage or blade or lining:
            mesh.normals_split_custom_set([(0,0,0)]*len(mesh.loops))
        changed[key]=obj
    # Botanical shader for the editable Blender asset, using the imported CC0 maps.
    if name=='lettuce':
        for obj in changed.values():
            if len(obj.data.vertices)>20000:
                for mat in obj.data.materials:
                    bsdf=next((n for n in mat.node_tree.nodes if n.type=='BSDF_PRINCIPLED'),None)
                    if bsdf:
                        bsdf.inputs['Subsurface Weight'].default_value=.07
                        bsdf.inputs['Subsurface Radius'].default_value=(.7,1,.35)
                        bsdf.inputs['Subsurface Scale'].default_value=.006
                        bsdf.inputs['Roughness'].default_value=.72
    stats=write_geometry_glb(name,changed)
    # Isolated edit file: no default cube and no other project assets.
    scene['source']='GrowNerve original procedural meshes, imported through glTF'
    scene['cc0_textures']='ambientCG; see frontend/public/textures/cc0/LICENSE.md'
    scene['refined_geometry_count']=len(changed)
    bpy.data.libraries.write(str(ROOT/'assets/blender'/f'{name}.blend'),{scene},compress=True)
    print(json.dumps({'model':name,'refined':len(changed),'bytes':(DEST/f'{name}.glb').stat().st_size,'geometry':stats}))
    return scene
