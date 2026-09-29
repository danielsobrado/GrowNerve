"""Blender-authored interchangeable LED fixtures, pots, and soil insert."""
exec(compile(open('F:/Development/GrowNerve/assets/blender/build_tower.py').read().split('# Tapered reservoir')[0],'tower_helpers','exec'))
exec(compile((ROOT/'assets/blender/tower_sensors.py').read_text().split('# Reservoir instrument hub')[0].replace('segments=3','segments=1'),'sensor_helpers','exec'))
import random
scene.name='GrowNerve - Lights and soil pots'
ceramic=material('Terracotta ceramic','a86040',.8)
fabric=material('Fabric grow bag | ambientCG CC0','343d34',.95,0,'fabric')
soilmat=material('Soil dark loam','302219',.95)
perlite=material('Perlite granules','d1cbb4',.9)
emitter=material('LED emitter','fff0c8',.35)
red=material('LED red emitter','ed5651',.4)
modules=[]
def module(role):
    root=group(role);modules.append((root,role));return root

for style,mat in [('nursery',dark),('fabric',fabric),('ceramic',ceramic)]:
    root=module('pot_'+style)
    lathe(style+' hollow pot',[(.015,.012),(.105,.012),(.111,.023),(.147,.27),(.15,.29),(.145,.3),(.136,.3),(.135,.281),(.099,.032),(.015,.032),(.015,.012)],mat,root)
    ring(style+' rim',.143,.006,mat,root,(0,0,.294))
    ring(style+' foot',.108,.004,mat,root,(0,0,.016))
    if style=='nursery':
        for n in range(12):
            a=n*math.pi/6
            lo=Vector((.111*math.cos(a),.111*math.sin(a),.035));hi=Vector((.142*math.cos(a),.142*math.sin(a),.268))
            cylinder('Molded reinforcement rib',.002,(hi-lo).length,mat,root,lo,hi-lo)
    elif style=='fabric':
        for side in (-1,1):
            cable('Woven lifting handle',[(side*.14,0,.245),(side*.19,0,.30),(side*.18,0,.35),(side*.142,0,.275)],root,.009)
        for z in (.04,.265):ring('Stitched seam',.108+(z-.025)*.15,.0015,grey,root,(0,0,z))
    else:
        ring('Ceramic glazed lip',.148,.005,ceramic,root,(0,0,.281))
        lathe('Ceramic saucer',[(0,.002),(.148,.002),(.16,.01),(.16,.025),(.152,.028),(.146,.012),(0,.012)],ceramic,root)

root=module('soil');cylinder('Soil surface',.13,.01,soilmat,root,(0,0,0))
rng=random.Random(71)
for i in range(90):
    a=rng.random()*math.tau;r=.123*math.sqrt(rng.random())
    grain=cylinder('Perlite' if i%4==0 else 'Soil aggregate',rng.uniform(.0015,.004),rng.uniform(.002,.005),perlite if i%4==0 else soilmat,root,(r*math.cos(a),r*math.sin(a),.009))
    grain.scale.y=rng.uniform(.6,1.3)

for style in ('panel','bar','multi_bar'):
    root=module('led_'+style)
    strips=[0] if style=='bar' else [-.19,-.114,-.038,.038,.114,.19] if style=='multi_bar' else [-.14,-.07,0,.07,.14]
    if style=='panel':sensor_box('Panel aluminum backplate',(.55,.38,.017),(0,0,.025),grey,root)
    if style=='multi_bar':
        for x in (-.22,.22):sensor_box('Cross rail',(.025,.46,.025),(x,0,.047),grey,root)
    for y in strips:
        sensor_box('Extruded LED rail',(.55,.037,.023),(0,y,.017),grey,root)
        sensor_box('White LED circuit board',(.52,.027,.004),(0,y,.003),white,root)
        for j in range(18):
            sensor_box('LED diode',(.012,.016,.003),(-.245+j*.029,y,-.0005),red if j%9==0 else emitter,root)
        for x in (-.28,.28):sensor_box('Rail end cap',(.015,.041,.028),(x,y,.017),dark,root)
        for dx in (-.012,0,.012):sensor_box('Cooling fin',(.51,.002,.012),(0,y+dx,.034),grey,root)
    sensor_box('Constant-current driver',(.15,.06,.042),(0,0,.068),dark,root)
    for x in (-.18,.18):
        ring('Suspension eye',.009,.0025,steel,root,(x,0,.058),(0,1,0))
    cable('Driver cable',[(.075,0,.075),(.14,.02,.075),(.19,.02,.038)],root,.002)

# Compact each option into one mesh with separate material primitives.
bpy.context.view_layer.update()
export_scene=bpy.data.scenes.new('Grow equipment modules')
for root,role in modules:
    vertices=[];faces=[];uvs=[];mats=[];indices=[]
    for obj in root.children_recursive:
        if obj.type!='MESH':continue
        transform=root.matrix_world.inverted()@obj.matrix_world;offset=len(vertices)
        vertices.extend(transform@v.co for v in obj.data.vertices)
        if obj.active_material not in mats:mats.append(obj.active_material)
        for poly in obj.data.polygons:
            faces.append(tuple(offset+i for i in poly.vertices));indices.append(mats.index(obj.active_material))
            uvs.extend(tuple(obj.data.uv_layers.active.data[i].uv) for i in poly.loop_indices)
    mesh=bpy.data.meshes.new(role);mesh.from_pydata(vertices,[],faces);mesh.update()
    for mat in mats:mesh.materials.append(mat)
    uv=mesh.uv_layers.new(name='UVMap')
    for i,co in enumerate(uvs):uv.data[i].uv=co
    for i,p in enumerate(mesh.polygons):p.material_index=indices[i];p.use_smooth=True
    obj=bpy.data.objects.new(role,mesh);export_scene.collection.objects.link(obj);obj['equipmentModule']=role
bpy.context.window.scene=export_scene
for obj in export_scene.objects:obj.select_set(True)
destination=ROOT/'frontend/public/models/blender/grow-options.glb'
bpy.ops.export_scene.gltf(filepath=str(destination),export_format='GLB',export_extras=True,use_selection=True,use_active_scene=True)
bpy.context.window.scene=scene
for i,(root,role) in enumerate(modules):
    if role.startswith('pot'):root.location.x=(i-1)*.5
    elif role=='soil':root.location.z=.25
    else:root.location=((i-5)*.7,0,.85);root.rotation_euler.x=-1.0
bpy.data.libraries.write(str(ROOT/'assets/blender/grow-options.blend'),{scene},compress=True)
print('Equipment options exported:',destination.stat().st_size,'bytes')
