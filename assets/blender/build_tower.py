"""Reference-inspired modular hydroponic tower. All dimensions are illustrative metres."""
import bpy, math
from pathlib import Path
from mathutils import Vector

ROOT=Path('F:/Development/GrowNerve')
scene=bpy.data.scenes.new('GrowNerve - Hydroponic tower')
bpy.context.window.scene=scene

def linear(v): return v/12.92 if v<=.04045 else ((v+.055)/1.055)**2.4
def material(name,color,roughness=.5,metallic=0,surface=None):
    m=bpy.data.materials.new(name);m.use_nodes=True
    rgb=tuple(linear(int(color[i:i+2],16)/255) for i in (0,2,4))
    m.diffuse_color=(*rgb,1)
    bsdf=next(n for n in m.node_tree.nodes if n.type=='BSDF_PRINCIPLED')
    bsdf.inputs['Base Color'].default_value=(*rgb,1)
    bsdf.inputs['Roughness'].default_value=roughness
    bsdf.inputs['Metallic'].default_value=metallic
    if surface:
        tex=m.node_tree.nodes.new('ShaderNodeTexImage')
        tex.image=bpy.data.images.load(str(ROOT/f'frontend/public/textures/cc0/{surface}/roughness-512.jpg'),check_existing=True)
        tex.image.colorspace_settings.name='Non-Color'
        m.node_tree.links.new(tex.outputs['Color'],bsdf.inputs['Roughness'])
        normal=m.node_tree.nodes.new('ShaderNodeTexImage')
        normal.image=bpy.data.images.load(str(ROOT/f'frontend/public/textures/cc0/{surface}/normalgl-512.jpg'),check_existing=True)
        normal.image.colorspace_settings.name='Non-Color'
        mapping=m.node_tree.nodes.new('ShaderNodeNormalMap');mapping.inputs['Strength'].default_value=.055 if surface=='metal' else .12
        m.node_tree.links.new(normal.outputs['Color'],mapping.inputs['Color'])
        m.node_tree.links.new(mapping.outputs['Normal'],bsdf.inputs['Normal'])
        m['texture_license']='CC0';m['texture_source']='ambientCG; frontend/public/textures/cc0/LICENSE.md'
    return m

grey=material('Brushed column | ambientCG CC0','a8ada9',.48,.8,'metal')
lime=material('Lime polymer | ambientCG CC0','91dc16',.48,0,'plastic')
tank=material('Reservoir polymer | ambientCG CC0','555e58',.7,0,'plastic')
dark=material('Graphite elastomer','19231d',.8)
steel=material('Stainless fittings','b8c0b9',.3,.85)
def group(name):
    o=bpy.data.objects.new(name,None);scene.collection.objects.link(o);return o
base=group('Tower_Base');tier=group('Tower_Tier');crown=group('Tower_Crown')

def lathe(name,profile,mat,parent,position=(0,0,0),axis=(0,0,1),segments=64):
    verts=[];faces=[];coords=[]
    for j,(r,z) in enumerate(profile):
        for i in range(segments+1):
            a=2*math.pi*i/segments;verts.append((r*math.cos(a),r*math.sin(a),z));coords.append((i/segments,j/(len(profile)-1)))
    for j in range(len(profile)-1):
        for i in range(segments):
            k=j*(segments+1)+i;faces.append((k,k+1,k+segments+2,k+segments+1))
    mesh=bpy.data.meshes.new(name);mesh.from_pydata(verts,[],faces);mesh.update()
    uv=mesh.uv_layers.new(name='UVMap')
    for poly in mesh.polygons:
        poly.use_smooth=True
        for li in poly.loop_indices:uv.data[li].uv=coords[mesh.loops[li].vertex_index]
    o=bpy.data.objects.new(name,mesh);scene.collection.objects.link(o);o.parent=parent;o.location=position
    o.rotation_euler=Vector(axis).to_track_quat('Z','Y').to_euler();mesh.materials.append(mat)
    return o
def ring(name,radius,tube,mat,parent,position,axis=(0,0,1)):
    profile=[(radius+tube*math.cos(i*2*math.pi/12),tube*math.sin(i*2*math.pi/12)) for i in range(13)]
    return lathe(name,profile,mat,parent,position,axis)
def cylinder(name,radius,height,mat,parent,position,axis=(0,0,1)):
    return lathe(name,[(0,0),(radius*.95,0),(radius,min(.003,height*.2)),(radius,height-min(.003,height*.2)),(radius*.95,height),(0,height)],mat,parent,position,axis,12 if radius<.01 else 48)

# Tapered reservoir, raised lid, drain and service fittings.
lathe('Tapered reservoir',[(0,.02),(.205,.02),(.224,.036),(.242,.085),(.286,.27),(.321,.365),(.322,.398),(.307,.409),(.297,.398),(.278,.29),(.218,.06),(0,.06)],tank,base)
lathe('Lime removable lid',[(.128,.403),(.322,.403),(.345,.412),(.349,.431),(.341,.448),(.136,.448),(.128,.437),(.128,.403)],lime,base)
ring('Reservoir lid gasket',.328,.005,dark,base,(0,0,.405))
lathe('Column lower coupler',[(.122,.431),(.137,.431),(.137,.515),(.128,.52),(.122,.515),(.122,.431)],grey,base)
for x,label in [(-.19,'Nutrient fill'),(.19,'Inspection')]:
    cylinder(label+' socket',.048,.025,steel,base,(x,-.155,.445))
    cylinder(label+' cap',.039,.016,dark,base,(x,-.155,.472))
    ring(label+' seal',.036,.0025,lime,base,(x,-.155,.487))
    for n in range(12):
        a=n*math.pi/6
        cylinder(label+' grip',.002,.013,steel,base,(x+.042*math.cos(a),-.155+.042*math.sin(a),.46))
for n in range(3):
    a=n*2*math.pi/3
    cylinder('Rubber foot',.045,.025,dark,base,(.18*math.cos(a),.18*math.sin(a),0))
cylinder('Drain fitting',.023,.045,steel,base,(0,-.247,.1),(0,-1,0))
cylinder('Drain plug',.025,.012,dark,base,(0,-.292,.1),(0,-1,0))
# Molded base bumper and lid tooling lines catch highlights at product scale.
ring('Lower protective bumper',.231,.006,dark,base,(0,0,.06))
ring('Lid perimeter tooling line',.344,.0018,grey,base,(0,0,.437))
for z in (.464,.486,.508):
    ring('Coupler grip rib',.138,.0022,steel,base,(0,0,z))
for n in range(6):
    a=n*math.pi/3
    cylinder('Lid captive screw',.0045,.002,steel,base,(.304*math.cos(a),.304*math.sin(a),.449))
# External sight tube follows the tapered reservoir; a dark sleeve makes it readable.
gauge_axis=Vector((0,-.056,.20)); gauge_start=Vector((.075,-.25,.115))
cylinder('Sight gauge protective sleeve',.014,gauge_axis.length,dark,base,gauge_start,gauge_axis)
cylinder('Sight gauge inset',.008,gauge_axis.length*.83,steel,base,gauge_start+Vector((0,-.01,.018)),gauge_axis)
for i in range(7):
    p=gauge_start+gauge_axis*(.12+i*.115)+Vector((-.027,-.017,0))
    cylinder('Sight gauge graduation',.0015,.016,lime,base,p,(1,0,0))
for end in (gauge_start,gauge_start+gauge_axis):
    cylinder('Sight gauge ferrule',.019,.009,grey,base,end,(0,-1,0))

# One repeatable tier. Hollow sockets and slotted baskets remain readable when empty.
lathe('Tier column',[(.122,-.15),(.128,-.15),(.13,-.148),(.13,-.144),(.13,.144),(.13,.148),(.128,.15),(.122,.15),(.122,-.15)],grey,tier)
ring('Tier seam',.13,.0028,steel,tier,(0,0,-.146))
ring('Tier joint gasket',.1305,.0015,dark,tier,(0,0,-.138))
for port in range(3):
    a=port*2*math.pi/3
    radial=Vector((math.sin(a),-math.cos(a),0))
    axis=radial*.75+Vector((0,0,.661438))
    start=radial*.093+Vector((0,0,-.08))
    tip=start+axis*.17
    ring(f'Port {port+1} collar gasket',.064,.003,dark,tier,tip-axis*.012,axis)
    lathe(f'Port {port+1} hollow neck',[(.046,0),(.056,0),(.065,.15),(.065,.17),(.055,.17),(.048,.015),(.046,0)],grey,tier,start,axis,48)
    lathe(f'Port {port+1} lime collar',[(.046,-.009),(.069,-.009),(.075,-.003),(.075,.009),(.069,.015),(.049,.015),(.046,.009),(.046,-.009)],lime,tier,tip,axis,64)
    ring(f'Port {port+1} inner lip',.051,.004,lime,tier,tip+axis*.013,axis)
    ring(f'Port {port+1} collar edge',.073,.0015,lime,tier,tip+axis*.008,axis)
    basket=group(f'Port {port+1} slotted basket');basket.parent=tier;basket.location=tip;basket.rotation_euler=axis.to_track_quat('Z','Y').to_euler()
    for height,radius in [(-.085,.033),(-.045,.04),(-.007,.048)]:
        ring('Basket hoop',radius,.0025,lime,basket,(0,0,height))
    for n in range(16):
        b=n*math.pi/8
        lo=Vector((.033*math.cos(b),.033*math.sin(b),-.086))
        hi=Vector((.048*math.cos(b),.048*math.sin(b),-.004))
        cylinder('Basket slat',.0022,(hi-lo).length,lime,basket,lo,hi-lo)
    for n in range(8):
        b=n*math.pi/4
        local=Vector((.071*math.cos(b),.071*math.sin(b),0))
        cylinder('Collar locking nub',.006,.009,lime,basket,local)
    cylinder('Basket base',.034,.003,dark,basket,(0,0,-.089))

# Top service cap and central feed connection.
lathe('Top cap',[(.12,0),(.134,0),(.135,.032),(.128,.044),(0,.044),(0,.036),(.12,.036),(.12,0)],grey,crown)
ring('Cap seal',.131,.003,dark,crown,(0,0,.004))
cylinder('Feed hose gland',.014,.02,steel,crown,(0,0,.043))
cylinder('Feed hose plug',.009,.012,dark,crown,(0,0,.063))
scene['design']='Reference-inspired six-level, three-sites-per-level tower; illustrative geometry'
scene['tiers']=6;scene['sites_per_tier']=3
scene['texture_license']='CC0 ambientCG; see frontend/public/textures/cc0/LICENSE.md'

exec(compile((ROOT/'assets/blender/tower_sensors.py').read_text(),'tower_sensors.py','exec'))

# Export the three reusable modules at their common origin.
destination=ROOT/'frontend/public/models/blender/hydroponic-tower.glb'
bpy.context.view_layer.update()
export_scene=bpy.data.scenes.new('Hydroponic tower modules')
for root,role in [(base,'base'),(tier,'tier'),(crown,'crown')]:
    vertices=[];faces=[];uvs=[];materials=[];face_materials=[]
    for obj in root.children_recursive:
        if obj.type!='MESH': continue
        transform=root.matrix_world.inverted()@obj.matrix_world
        offset=len(vertices);vertices.extend([transform@v.co for v in obj.data.vertices])
        mat=obj.active_material
        if mat not in materials:materials.append(mat)
        for poly in obj.data.polygons:
            faces.append(tuple(offset+i for i in poly.vertices));face_materials.append(materials.index(mat))
            uvs.extend([tuple(obj.data.uv_layers.active.data[i].uv) for i in poly.loop_indices])
    mesh=bpy.data.meshes.new('Tower '+role);mesh.from_pydata(vertices,[],faces);mesh.update()
    for mat in materials:mesh.materials.append(mat)
    uv=mesh.uv_layers.new(name='UVMap')
    for i,co in enumerate(uvs):uv.data[i].uv=co
    for i,poly in enumerate(mesh.polygons):poly.material_index=face_materials[i];poly.use_smooth=True
    obj=bpy.data.objects.new('Tower module '+role,mesh);export_scene.collection.objects.link(obj);obj['towerModule']=role
bpy.context.window.scene=export_scene
for obj in export_scene.objects:obj.select_set(True)
bpy.ops.export_scene.gltf(filepath=str(destination),export_format='GLB',export_extras=True,use_selection=True,use_active_scene=True)
bpy.context.window.scene=scene

# Arrange six staggered tiers for the editable assembled model and preview.
def descendants(root):
    return [root]+list(root.children_recursive)
tier.location.z=.65
for level in range(1,6):
    mapping={}
    for source in descendants(tier):
        duplicate=source.copy();scene.collection.objects.link(duplicate);mapping[source]=duplicate
    for source,duplicate in mapping.items():
        duplicate.parent=mapping.get(source.parent,source.parent)
    root=mapping[tier];root.name=f'Tower_Tier_{level+1}';root.location.z=.65+level*.3;root.rotation_euler.z=(level%2)*math.pi/3
crown.location.z=2.3
scene['assembled']=True
bpy.data.libraries.write(str(ROOT/'assets/blender/hydroponic-tower.blend'),{scene},compress=True)
print('Tower complete:',len(scene.objects),'objects;',destination.stat().st_size,'bytes')
