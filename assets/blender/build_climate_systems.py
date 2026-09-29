"""Illustrative climate and irrigation assemblies; reusable browser modules."""
exec(compile(open('F:/Development/GrowNerve/assets/blender/build_tower.py').read().split('# Tapered reservoir')[0],'tower_helpers','exec'))
exec(compile((ROOT/'assets/blender/tower_sensors.py').read_text().split('# Reservoir instrument hub')[0].replace('segments=3','segments=1'),'sensor_helpers','exec'))
scene.name='GrowNerve - Climate and irrigation'
blue=material('Water tank blue polymer | ambientCG CC0','4e8396',.4,0,'plastic')
modules=[]
def module(role):
    root=group(role);modules.append((root,role));return root
def rod(name,start,end,radius,mat,parent):
    a,b=Vector(start),Vector(end);return cylinder(name,radius,(b-a).length,mat,parent,a,b-a)
def screw(position,parent):cylinder('Captive screw',.003,.003,steel,parent,position,(0,-1,0))

for style in ('clip','inline'):
    root=module('fan_'+style)
    depth=.075 if style=='clip' else .23
    lathe('Fan housing',[(.14,0),(.153,0),(.159,.008),(.159,depth-.008),(.15,depth),(.14,depth),(.14,0)],white if style=='clip' else grey,root,(0,-.04,.32),(0,1,0),48)
    for y in (-.045,depth-.038):
        for radius in (.04,.075,.11,.145):ring('Wire guard',radius,.0018,steel,root,(0,y,.32),(0,1,0))
        for n in range(8):
            a=n*math.pi/4;rod('Guard spoke',(0,y,.32),(.145*math.cos(a),y,.32+.145*math.sin(a)),.0015,steel,root)
    sensor_box('Fan mounting foot',(.18,.13,.025),(0,.05,.02),dark,root)
    rod('Fan support',(0,.05,.025),(0,.05,.20),.016,steel,root)
    if style=='clip':
        sensor_box('Spring clamp upper jaw',(.10,.17,.017),(0,.05,.08),white,root)
        cylinder('Clamp hinge',.018,.09,steel,root,(-.045,.08,.06),(1,0,0))
    else:
        for y in (-.035,.18):ring('Duct clamp band',.159,.003,steel,root,(0,y,.32),(0,1,0))
        sensor_box('Inline motor terminal box',(.10,.08,.045),(0,.075,.50),dark,root)
    cable('Fan power lead',[(.06,.05,.18),(.12,.13,.12),(.17,.16,.018)],root,.003)
    rotor=module('rotor_'+style)
    cylinder('Motor hub',.036,.035,dark,rotor,(0,0,0),(0,1,0))
    for n in range(5):
        blade=sensor_box('Pitched impeller blade',(.09,.008,.045),(.074,0,0),lime if style=='clip' else dark,rotor)
        a=n*math.tau/5
        blade.location=(.074*math.cos(a),.012,.074*math.sin(a));blade.rotation_euler=(.28,-a,.15)

for style in ('ultrasonic','evaporative'):
    root=module('humidifier_'+style)
    sensor_box('Humidifier base',(.25,.22,.075),(0,0,.047),white,root)
    if style=='ultrasonic':
        lathe('Removable water tank',[(0,.078),(.10,.078),(.113,.09),(.113,.29),(.104,.308),(0,.308)],blue,root)
        cylinder('Tank lid',.107,.02,white,root,(0,0,.307))
        lathe('Directional mist nozzle',[(.012,0),(.025,0),(.025,.035),(.02,.045),(.012,.045),(.012,0)],white,root,(.035,0,.325),segments=32)
        for z in (.13,.18,.23,.28):sensor_box('Tank level mark',(.014,.002,.0015),(.01,-.114,z),white,root)
    else:
        sensor_box('Evaporative wick cabinet',(.28,.23,.30),(0,0,.23),white,root)
        sensor_box('Wick intake dark backing',(.23,.004,.23),(0,-.118,.23),dark,root)
        for i in range(15):sensor_box('Wick intake louvre',(.23,.012,.006),(0,-.125,.12+i*.015),white,root)
        for x in (-.11,.11):sensor_box('Reservoir level window',(.012,.005,.16),(x,-.132,.22),blue,root)
        for x in (-.09,-.06,-.03,0,.03,.06,.09):sensor_box('Top exhaust slot',(.012,.16,.003),(x,0,.382),dark,root)
    sensor_box('Humidifier controls',(.115,.005,.025),(0,-.115,.052),dark,root)
    sensor_label('HUMIDITY',(-.049,-.119,.047),.012,root)
    for x in (-.10,.10):
        for y in (-.08,.08):cylinder('Isolation foot',.015,.01,dark,root,(x,y,0))

for style in ('drip','ring'):
    root=module('irrigation_'+style)
    # Supply tank, pump/filter train, timer and separately valved two-zone manifold.
    sensor_box('Irrigation supply tank',(.24,.24,.29),(-.36,.10,.155),blue,root)
    sensor_box('Supply tank lid',(.25,.25,.017),(-.36,.10,.308),white,root)
    cylinder('Tank fill cap',.035,.016,dark,root,(-.36,.10,.317))
    sensor_box('Pump mounting base',(.15,.12,.014),(-.13,.10,.018),dark,root)
    cylinder('Diaphragm pump motor',.035,.12,grey,root,(-.18,.10,.065),(1,0,0))
    cylinder('Pump head',.043,.027,dark,root,(-.065,.10,.065),(1,0,0))
    cable('Tank suction line',[(-.24,.10,.055),(-.21,.10,.045),(-.18,.10,.065)],root,.007)
    cylinder('Inline filter bowl',.023,.07,white,root,(.01,.10,.04))
    ring('Filter union',.024,.003,dark,root,(.01,.10,.105))
    cable('Pump discharge',[(-.04,.10,.065),(-.02,.10,.12),(.01,.10,.12)],root,.006)
    rod('Manifold header',(.01,.10,.12),(.29,.10,.12),.008,steel,root)
    sensor_box('Irrigation timer enclosure',(.14,.045,.105),(-.35,-.046,.23),white,root)
    sensor_box('Timer face',(.126,.003,.088),(-.35,-.07,.23),dark,root)
    sensor_label('IRRIGATION',(-.405,-.073,.248),.014,root)
    sensor_label('ZONE 1 / 2',(-.40,-.073,.222),.012,root)
    cable('Pump control cable',[(-.30,-.05,.19),(-.22,-.04,.075),(-.12,.06,.075)],root,.0025)
    for index,x in enumerate((.12,.27)):
        cylinder('Solenoid valve',.015,.033,steel,root,(x,.10,.128))
        sensor_box('Solenoid coil',(.034,.035,.033),(x,.10,.175),dark,root)
        cylinder('Valve manual override',.007,.008,lime,root,(x,.10,.192))
        cable('Valve control lead',[(-.30,-.05,.20),(-.19,0,.20),(x,.12,.20),(x,.10,.19)],root,.002)
        endx=.08+index*.27
        cable('Zone distribution tubing',[(x,.10,.13),(x,-.01,.08),(endx,-.17,.035),(endx,-.24,.05)],root,.004)
        if style=='drip':
            rod('Dripper stake',(endx,-.24,.012),(endx,-.24,.09),.0035,dark,root)
            cylinder('Adjustable dripper',.012,.016,lime,root,(endx,-.24,.085))
            for n in range(6):
                a=n*math.tau/6;rod('Dripper grip',(endx+.011*math.cos(a),-.24+.011*math.sin(a),.087),(endx+.011*math.cos(a),-.24+.011*math.sin(a),.10),.0015,dark,root)
        else:
            ring('Irrigation distribution ring',.09,.005,dark,root,(endx,-.32,.05))
            for n in range(8):
                a=n*math.tau/8;cylinder('Ring emitter',.003,.008,lime,root,(endx+.087*math.cos(a),-.32+.087*math.sin(a),.047))

# Reuse the established compactor; only the file names and presentation differ.
export_code=(ROOT/'assets/blender/build_grow_options.py').read_text().split('# Compact each option')[1].partition('\n')[2].split('for i,(root,role) in enumerate(modules):')[0]
exec(compile(export_code.replace('grow-options.glb','climate-systems.glb'),'export_systems','exec'))
for i,(root,role) in enumerate(modules):
    if role.startswith('rotor_'):continue
    root.location=((i%4)*.65-1, (i//4)*.8, 0)
for style in ('clip','inline'):
    body=next(r for r,k in modules if k=='fan_'+style)
    rotor=next(r for r,k in modules if k=='rotor_'+style)
    rotor.location=body.location+Vector((0,-.015,.32))
bpy.data.libraries.write(str(ROOT/'assets/blender/climate-systems.blend'),{scene},compress=True)
print('Climate systems exported:',destination.stat().st_size,'bytes')
