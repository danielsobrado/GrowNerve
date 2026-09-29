"""Illustrative sensor hardware, executed within build_tower.py before export."""
import bmesh

white=material('Sensor ivory polymer | ambientCG CC0','e4e8df',.6,0,'plastic')
blue=material('pH identification blue','388bcb',.45)
orange=material('EC identification amber','efa93d',.45)

def sensor_box(name,size,position,mat,parent):
    mesh=bpy.data.meshes.new(name)
    bm=bmesh.new();bmesh.ops.create_cube(bm,size=1)
    for v in bm.verts:
        v.co=Vector(tuple(v.co[i]*size[i] for i in range(3)))
    bmesh.ops.bevel(bm,geom=list(bm.edges),offset=min(.004,min(size)*.2),segments=3)
    bm.to_mesh(mesh);bm.free();mesh.uv_layers.new(name='UVMap')
    mesh.materials.append(mat)
    obj=bpy.data.objects.new(name,mesh);scene.collection.objects.link(obj)
    obj.parent=parent;obj.location=position
    return obj

def sensor_label(body,position,size,parent):
    curve=bpy.data.curves.new('Label '+body,'FONT');curve.body=body;curve.size=size
    curve.extrude=.00015;curve.resolution_u=2
    obj=bpy.data.objects.new('Label '+body,curve);scene.collection.objects.link(obj)
    obj.parent=parent;obj.location=position;obj.rotation_euler=(math.pi/2,0,0)
    bpy.context.view_layer.update()
    mesh=bpy.data.meshes.new_from_object(obj.evaluated_get(bpy.context.evaluated_depsgraph_get()))
    replacement=bpy.data.objects.new(obj.name+' mesh',mesh);scene.collection.objects.link(replacement)
    replacement.parent=parent;replacement.matrix_basis=obj.matrix_basis.copy()
    bpy.data.objects.remove(obj,do_unlink=True)
    mesh.uv_layers.new(name='UVMap');mesh.materials.append(white)

def cable(name,points,parent,radius=.003):
    # Rounded polyline, with enough samples that bends are visually gentle.
    curve=bpy.data.curves.new(name,'CURVE');curve.dimensions='3D';curve.bevel_depth=radius;curve.bevel_resolution=2
    spline=curve.splines.new('BEZIER');spline.bezier_points.add(len(points)-1)
    valid=[i.identifier for i in bpy.types.BezierSplinePoint.bl_rna.properties['handle_left_type'].enum_items]
    for point,co in zip(spline.bezier_points,points):
        point.co=co
        if 'AUTO' in valid:point.handle_left_type='AUTO';point.handle_right_type='AUTO'
    obj=bpy.data.objects.new(name,curve);scene.collection.objects.link(obj);obj.parent=parent
    bpy.context.view_layer.update()
    mesh=bpy.data.meshes.new_from_object(obj.evaluated_get(bpy.context.evaluated_depsgraph_get()))
    replacement=bpy.data.objects.new(name+' mesh',mesh);scene.collection.objects.link(replacement);replacement.parent=parent
    bpy.data.objects.remove(obj,do_unlink=True)
    mesh.uv_layers.new(name='UVMap');mesh.materials.append(dark)

# Reservoir instrument hub: separate from the fill and inspection caps.
sensor_box('Sensor hub mounting bracket',(.10,.05,.055),(.285,0,.48),steel,base)
sensor_box('Sensor hub enclosure',(.19,.07,.135),(.335,-.065,.565),white,base)
sensor_box('Sensor hub face gasket',(.176,.004,.12),(.335,-.102,.565),dark,base)
sensor_box('Sensor hub display',(.142,.003,.067),(.335,-.105,.58),dark,base)
sensor_label('SENSOR HUB',(.27,-.108,.59),.014,base)
sensor_label('pH  EC  TEMP  LVL',(.267,-.108,.565),.009,base)
for x in (.257,.413):
    for z in (.515,.615):cylinder('Hub captive screw',.003,.003,steel,base,(x,-.107,z),(0,-1,0))

# Four distinct immersed instruments. Shafts extend into the reservoir.
for name,x,y,accent in [('pH',.22,.075,blue),('EC',.14,.20,orange),('TEMP',-.04,.25,steel),('LEVEL',-.20,.16,lime)]:
    root=group('Sensor '+name);root.parent=base;root['sensor_type']=name
    cylinder(name+' bulkhead gland',.02,.028,dark,root,(x,y,.439))
    cylinder(name+' locking ring',.023,.009,steel,root,(x,y,.465))
    cylinder(name+' connector head',.016,.038,accent,root,(x,y,.474))
    cylinder(name+' cable strain relief',.009,.023,dark,root,(x,y,.51))
    radius=.006 if name=='TEMP' else .009
    cylinder(name+' immersion shaft',radius,.235,steel if name in ('TEMP','LEVEL') else dark,root,(x,y,.22))
    if name=='pH':
        cylinder('pH electrode bulb',.006,.022,white,root,(x,y,.199))
        for n in range(4):
            a=n*math.pi/2;cylinder('pH bulb guard',.0018,.034,dark,root,(x+.008*math.cos(a),y+.008*math.sin(a),.196))
    elif name=='EC':
        for z in (.215,.237,.259,.281):cylinder('EC electrode ring',.0096,.01,steel,root,(x,y,z))
    elif name=='LEVEL':
        lathe('Level float',[(.01,0),(.026,0),(.03,.006),(.03,.035),(.026,.041),(.01,.041),(.01,0)],white,root,(x,y,.245),segments=32)
        ring('Level float stop',.012,.002,steel,root,(x,y,.231))
    cable(name+' shielded lead',[(x,y,.53),(x,y,.58),(.28,.13,.59),(.34,-.02,.525)],root)

# Crown instruments follow the selected tower height; they are never duplicated per tier.
sensor_box('Air sensor standoff',(.27,.035,.023),(.195,.01,.059),steel,crown)
sensor_box('Humidity and air temperature sensor',(.095,.065,.13),(.30,.01,.095),white,crown)
sensor_box('Air sensor dark vent recess',(.077,.003,.072),(.30,-.024,.097),dark,crown)
for z in (.071,.082,.093,.104,.115,.126):
    sensor_box('Air sensor louvre',(.077,.008,.004),(.30,-.027,z),white,crown)
sensor_box('Air sensor label plate',(.08,.004,.018),(.30,-.026,.045),dark,crown)
sensor_label('RH / TEMP',(.266,-.029,.04),.011,crown)
cable('Air sensor short lead',[(.30,.025,.026),(.25,.06,.015),(.09,.065,.053)],crown)
# Upward-facing PAR/light sensor with a white cosine diffuser.
sensor_box('Light sensor arm',(.20,.03,.018),(-.17,0,.055),steel,crown)
cylinder('PAR sensor body',.032,.033,dark,crown,(-.25,0,.058))
cylinder('PAR cosine diffuser',.026,.008,white,crown,(-.25,0,.091))
ring('PAR bezel',.029,.002,steel,crown,(-.25,0,.091))
cable('PAR sensor lead',[(-.25,.025,.075),(-.20,.065,.056),(-.07,.065,.052)],crown)
scene['sensor_types']='Air humidity/temperature, pH, EC, water temperature, float level, PAR/light'
scene['sensor_note']='Illustrative hardware only; no live readings or manufacturer specifications'
