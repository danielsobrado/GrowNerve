"""Create a separate editable probe lineup from the assembled tower scene."""
import bpy
from pathlib import Path
from mathutils import Vector
ROOT=Path('F:/Development/GrowNerve')
source=bpy.context.scene
kit=bpy.data.scenes.new('GrowNerve - Water sensor kit')
for index,(kind,x,y) in enumerate([('pH',.22,.075),('EC',.14,.20),('TEMP',-.04,.25),('LEVEL',-.20,.16)]):
    original=next(o for o in source.objects if o.get('sensor_type')==kind)
    root=bpy.data.objects.new(kind+' probe',None);kit.collection.objects.link(root)
    root['sensor_type']=kind
    root.location=(index*.14-.21-x,-y,-.196)
    for child in original.children_recursive:
        if child.type!='MESH' or 'lead' in child.name:continue
        copy=child.copy();kit.collection.objects.link(copy);copy.parent=root
    curve=bpy.data.curves.new(kind+' caption','FONT');curve.body=kind;curve.size=.019
    caption=bpy.data.objects.new(kind+' caption',curve);kit.collection.objects.link(caption)
    caption.location=(index*.14-.235,-.05,-.016);caption.rotation_euler=(1.5707963,0,0)
    curve.materials.append(next(m for m in bpy.data.materials if m.name.startswith('Sensor ivory polymer')))
for original in source.objects:
    if original.type=='LIGHT':
        copy=original.copy();copy.data=original.data.copy();kit.collection.objects.link(copy)
        copy.location=Vector(original.location)*.3;copy.data.energy*=.09;copy.data.size*=.3
    elif original.name.startswith('Studio sweep'):
        copy=original.copy();kit.collection.objects.link(copy);copy.location.z=-.025
camera=bpy.data.objects.new('Probe camera',bpy.data.cameras.new('Probe camera'));kit.collection.objects.link(camera)
camera.location=(.45,-1.1,.65);target=Vector((0,0,.15))
camera.rotation_euler=(target-camera.location).to_track_quat('-Z','Y').to_euler();camera.data.lens=62
kit.camera=camera;kit.world=source.world
kit.render.engine=source.render.engine
kit.cycles.samples=48;kit.cycles.use_denoising=True
kit.render.resolution_x=1100;kit.render.resolution_y=850;kit.render.resolution_percentage=100
kit.render.filepath=str(ROOT/'assets/blender/previews/sensor-probes.png')
bpy.context.window.scene=kit
bpy.data.libraries.write(str(ROOT/'assets/blender/sensor-probes.blend'),{kit},compress=True)
bpy.ops.render.render(write_still=True)
bpy.context.window.scene=source
print('Saved four individual water probes and preview')
