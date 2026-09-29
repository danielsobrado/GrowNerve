"""Add a reusable studio setup to an existing refined asset, without changing its mesh export."""
import bpy
from pathlib import Path
from mathutils import Vector

ROOT=Path('F:/Development/GrowNerve')

def preview(name, render=True):
    path=ROOT/'assets/blender'/f'{name}.blend'
    with bpy.data.libraries.load(str(path),link=False) as (source,target):
        target.scenes=source.scenes[:1]
    scene=target.scenes[0]
    bpy.context.window.scene=scene
    # Match the application's illustrative proportions, including its flattened LED fixture.
    scales={'tent':(4,4,3),'reservoir':(2.5,2.5,1),'light':(2.4,1.4,.15),
            'fan':(.45,.35,.45),'lettuce':(.75,.75,.75)}
    roots=[o for o in scene.objects if o.parent is None and o.type=='EMPTY']
    for o in roots: o.scale=scales.get(name,(1,1,1))
    bpy.context.view_layer.update()
    points=[o.matrix_world@Vector(c) for o in scene.objects if o.type=='MESH' for c in o.bound_box]
    lo=Vector(tuple(min(p[i] for p in points) for i in range(3)))
    hi=Vector(tuple(max(p[i] for p in points) for i in range(3)))
    center=(lo+hi)/2; size=max(hi-lo)
    camera=bpy.data.objects.new('Preview camera',bpy.data.cameras.new('Preview camera'))
    scene.collection.objects.link(camera)
    direction=Vector((1.35,-2.2,1.6)).normalized()
    camera.location=center+direction*size*2.5
    camera.rotation_euler=(center-camera.location).to_track_quat('-Z','Y').to_euler()
    camera.data.lens=48
    scene.camera=camera
    light_types=[i.identifier for i in bpy.types.Light.bl_rna.properties['type'].enum_items]
    for label,offset,power in [('Key',(-2,-3,4),180),('Fill',(3,-1,2),80),('Rim',(0,3,3),140)]:
        light=bpy.data.lights.new(label,'AREA' if 'AREA' in light_types else light_types[0])
        light.energy=power*size**2; light.size=size*2
        obj=bpy.data.objects.new(label,light);scene.collection.objects.link(obj)
        obj.location=center+Vector(offset)*size
        obj.rotation_euler=(center-obj.location).to_track_quat('-Z','Y').to_euler()
    world=bpy.data.worlds.new('CC0 studio environment');world.use_nodes=True;scene.world=world
    bg=next(n for n in world.node_tree.nodes if n.type=='BACKGROUND')
    bg.inputs['Strength'].default_value=.45
    env=world.node_tree.nodes.new('ShaderNodeTexEnvironment')
    env.image=bpy.data.images.load(str(ROOT/'frontend/public/textures/cc0/studio_small_09_1k.hdr'),check_existing=True)
    env.image.pack()
    world.node_tree.links.new(env.outputs['Color'],bg.inputs['Color'])
    scene.render.film_transparent=True
    scene.render.resolution_x=1000;scene.render.resolution_y=850;scene.render.resolution_percentage=100
    destination=ROOT/'assets/blender/previews';destination.mkdir(exist_ok=True)
    scene.render.filepath=str(destination/f'{name}.png')
    for area in bpy.context.screen.areas:
        if area.type=='VIEW_3D':
            area.spaces.active.region_3d.view_location=center
            area.spaces.active.region_3d.view_distance=size*2.5
            area.spaces.active.region_3d.view_rotation=direction.to_track_quat('Z','Y')
            area.spaces.active.overlay.show_overlays=False
    bpy.data.libraries.write(str(path),{scene},compress=True)
    if render: bpy.ops.render.render(write_still=True)
    print('Saved',name,'size',tuple(hi-lo),'preview',scene.render.filepath)
