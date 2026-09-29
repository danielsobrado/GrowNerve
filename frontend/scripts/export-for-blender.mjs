import { chromium } from '@playwright/test';
import { mkdir,stat } from 'node:fs/promises';
const browser=await chromium.launch({headless:true});
const page=await browser.newPage();
page.on('pageerror',e=>console.error(e));
page.on('console',m=>console.log('browser:',m.text()));
await page.goto('http://127.0.0.1:5173/GrowNerve/scripts/blender-export.html');
await page.waitForFunction(()=>typeof window.exportModel==='function');
await page.waitForTimeout(8000);
await mkdir('../assets/blender/imports',{recursive:true});
for(const name of ['lettuce','tent','reservoir','light','fan','controller','pump']){
 console.log('Exporting',name);
 const pending=page.waitForEvent('download');
 await page.evaluate(name=>window.exportModel(name),name);
 const download=await pending;
 const target=`../assets/blender/imports/${name}.glb`;
 await download.saveAs(target);
 console.log(name,(await stat(target)).size);
}
await browser.close();
