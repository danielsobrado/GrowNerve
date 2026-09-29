"""Run after build_tower.py and preview('hydroponic-tower', render=False)."""
import bpy
from pathlib import Path
from mathutils import Vector

ROOT=Path('F:/Development/GrowNerve')
scene=bpy.context.scene
center=Vector((0,0,1.15))
camera=scene.camera
camera.location=(3.2,-5.4,3.0)
camera.rotation_euler=(center-camera.location).to_track_quat('-Z','Y').to_euler()
camera.data.lens=65
scene.render.resolution_x=1100
scene.render.resolution_y=1300
scene.render.film_transparent=False
# A matte sweep grounds the product with soft contact shadows.
mesh=bpy.data.meshes.new('Studio sweep')
mesh.from_pydata([(-200,-200,-.002),(200,-200,-.002),(200,200,-.002),(-200,200,-.002)],[],[(0,1,2,3)])
floor=bpy.data.objects.new('Studio sweep',mesh)
scene.collection.objects.link(floor)
mat=bpy.data.materials.new('Studio charcoal');mat.use_nodes=True
bsdf=next(n for n in mat.node_tree.nodes if n.type=='BSDF_PRINCIPLED')
bsdf.inputs['Base Color'].default_value=(.035,.046,.04,1)
bsdf.inputs['Roughness'].default_value=.82
mesh.materials.append(mat)
for obj in scene.objects:
    if obj.type=='LIGHT':
        obj.data.energy*=.8
world_bg=next(n for n in scene.world.node_tree.nodes if n.type=='BACKGROUND')
world_bg.inputs['Strength'].default_value=.28
try:
    scene.render.engine='CYCLES'
    scene.cycles.samples=48
    scene.cycles.use_denoising=True
except TypeError:
    pass
for area in bpy.context.screen.areas:
    if area.type=='VIEW_3D':
        space=area.spaces.active
        space.region_3d.view_location=center
        space.region_3d.view_distance=4.5
        space.region_3d.view_rotation=(camera.location-center).to_track_quat('Z','Y')
        choices=[i.identifier for i in space.shading.bl_rna.properties['color_type'].enum_items]
        if 'MATERIAL' in choices:space.shading.color_type='MATERIAL'
bpy.data.libraries.write(str(ROOT/'assets/blender/hydroponic-tower.blend'),{scene},compress=True)
bpy.ops.render.render(write_still=True)
print('Studio render saved:',scene.render.filepath)
