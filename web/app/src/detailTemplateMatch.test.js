import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { rankDetailTemplates, templateImage } from './detailTemplateMatch.js';
const catalog=JSON.parse(fs.readFileSync(new URL('./detailReferenceCatalog.json',import.meta.url)));
test('brand references match clothing and shoes without recommending unrelated categories',()=>{
 assert.equal(rankDetailTemplates(catalog,{title:'牛仔短外套'})[0].id,'B03');
 const shoes=rankDetailTemplates(catalog,{title:'轻量运动鞋',claimsText:'运动'});
 assert.equal(shoes[0].id,'B05');
 assert.ok(shoes.every(t=>t.category==='鞋履'));
 assert.equal(rankDetailTemplates(catalog,{title:'保湿面霜'}).length,0);
 for(const t of catalog){assert.ok(fs.existsSync(new URL(`../public${t.preview_url}`,import.meta.url)));assert.ok(!t.source_preview_url.includes('aimadeo'));}
});
test('selecting a catalog image reuses its uploaded asset and preserves selection',()=>{
 const refs=[{id:'old',original_name:'B01-a.jpg',created_at:'2026-01-01'},{id:'new',original_name:'B01-b.jpg',created_at:'2026-02-01'},{id:'other',original_name:'B03-a.jpg'}];
 assert.equal(templateImage(catalog[0],[]),undefined);
 assert.equal(templateImage(catalog[0],refs).id,'new');
 assert.equal(templateImage(catalog[0],refs,'old').id,'old');
});
