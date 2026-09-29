"""Extract individually placeable sensors from the existing Blender assemblies."""
import bpy
from pathlib import Path
ROOT=Path('F:/Development/GrowNerve')
original=bpy.context.scene
export=bpy.data.scenes.new('GrowNerve - Individual sensors')
def load_scene(name):
    with bpy.data.libraries.load(str(ROOT/'assets/blender'/f'{name}.blend'),link=False) as (_,target):target.scenes=[name_ for name_ in _.scenes][:1]
    return target.scenes[0]
def module(role,objects):
    root=bpy.data.objects.new(role,None);export.collection.objects.link(root);root['equipmentModule']=role
    for source in objects:
        if source.type!='MESH':continue
        obj=source.copy();export.collection.objects.link(obj);obj.parent=root;obj.matrix_world=source.matrix_world.copy()
    return root
kit=load_scene('sensor-probes');bpy.context.window.scene=kit;bpy.context.view_layer.update()
for root in list(kit.objects):
    if root.get('sensor_type'):module('sensor_'+root['sensor_type'].lower(),root.children_recursive)
tower=load_scene('hydroponic-tower');bpy.context.window.scene=tower;bpy.context.view_layer.update()
module('sensor_air',[o for o in tower.objects if o.name.startswith(('Humidity and air temperature','Air sensor','Label RH'))])
module('sensor_par',[o for o in tower.objects if o.name.startswith(('PAR sensor body','PAR cosine','PAR bezel','Light sensor arm'))])
bpy.context.window.scene=export
for obj in export.objects:obj.select_set(True)
bpy.ops.export_scene.gltf(filepath=str(ROOT/'frontend/public/models/blender/layout-sensors.glb'),export_format='GLB',export_extras=True,use_selection=True,use_active_scene=True)
bpy.data.libraries.write(str(ROOT/'assets/blender/layout-sensors.blend'),{export},compress=True)
bpy.context.window.scene=original
print('Exported six individual sensor modules')
