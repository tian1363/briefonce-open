import ImageModelPicker from "./ImageModelPicker.jsx";
import { useEffect, useRef, useState } from 'react';
import { ChevronDown, Loader2, Plus, Sparkles, Upload, X } from 'lucide-react';
import { listCharacters, uploadCharacter, generateCharacter } from './api.js';
import DownloadableImage from './DownloadableImage.jsx';
import './CharacterLibrary.css';

export const characterImage = (role) => `/api/characters/${role.id}/image`;
const defaults = { description:'自然真实的成年生活方式创作者', age:28, hair:'自然棕色中长发', outfit:'简洁日常休闲装', expression:'自然微笑', pose:'正面站立，双手自然放松', framing:'半身，完整头部入镜', background:'浅灰纯色背景', lighting:'柔和自然光', size:'720*1280', seed:42, negative:'' };
const presets = [
 {name:'电商·极简棚拍', description:'成年亚洲时装模特，五官清晰，比例自然，专业电商摄影质感', age:27,hair:'自然深色中长发',outfit:'无品牌标识的基础款单色服装',background:'浅灰无缝棚拍背景',lighting:'均匀柔和棚拍光'},
 {name:'电商·都市通勤', description:'成年都市通勤风格模特，形象亲和，站姿自然', age:32,hair:'利落深色发型',outfit:'简约通勤基础款',background:'干净的现代室内背景',lighting:'明亮自然窗光'},
 {name:'电商·户外运动', description:'成年户外运动风格模特，健康自然，身体比例真实', age:29,hair:'自然运动短发',outfit:'无标识基础运动装',background:'简洁户外背景',lighting:'柔和日光'},
 {name:'电商·轻奢质感', description:'成年时尚电商模特，克制优雅，真实摄影质感', age:30,hair:'整洁自然发型',outfit:'无品牌标识的高级感基础服饰',background:'米色简洁摄影棚',lighting:'柔和侧光'},
 {name:'运动·瑜伽自然',description:'原创成年女性运动模特，匀称轻运动体态，自然肤质，适合瑜伽与贴身运动服',age:28,hair:'深色头发低束',outfit:'无标识鼠尾草绿运动上衣与炭灰紧身裤',pose:'正面略侧站立，手臂自然离开腰线',framing:'全身，完整头部与鞋子入镜',background:'暖白无缝摄影棚',lighting:'均匀柔和棚拍光',negative:'品牌标识、文字、夸张妆容、过度修饰身材'},
 {name:'运动·力量训练',description:'原创成年女性运动模特，肩腿自然有力量感，真实身体比例，适合训练服',age:30,hair:'自然卷发束起',outfit:'无标识灰紫运动背心与黑色紧身裤',pose:'轻微三分之二侧身站立，双臂自然放松',framing:'全身，完整头部与鞋子入镜',background:'暖白无缝摄影棚',lighting:'柔和均匀棚拍光',negative:'品牌标识、文字、夸张妆容、极端肌肉'},
 {name:'运动·男士通勤',description:'原创成年男性模特，自然健身型体态，轻运动与都市休闲气质，适合运动通勤服',age:31,hair:'利落深色短发',outfit:'无标识海军蓝运动T恤与石板灰束脚裤',pose:'正面略侧自然站立，手臂与躯干分开',framing:'全身，完整头部与鞋子入镜',background:'暖白无缝摄影棚',lighting:'柔和均匀棚拍光',negative:'品牌标识、文字、极端肌肉、墨镜'},
 {name:'北美生活方式', description:'面向北美生活方式广告的成年女性创作者，自然真实的个人风格', age:28,hair:'棕色微卷长发',outfit:'米白色针织上衣'},
 {name:'运动分享', description:'成年男性运动内容创作者，自然健康的体态',age:30,hair:'黑色短发',outfit:'无品牌标识的运动上衣'},
 {name:'家居日常', description:'成年女性家居内容创作者，亲切自然',age:38,hair:'深棕色齐肩发',outfit:'浅蓝色休闲衬衫'},
 {name:'专业讲解', description:'成年男性产品讲解者，整洁自然',age:45,hair:'整洁深色短发',outfit:'简洁深灰色衬衫'},
];
const gallery = [
 {id:'01-east-asian-woman',name:'米色日常 · 女模特',description:'简约休闲',image:'/model-gallery/01-east-asian-woman.jpg'},
 {id:'02-black-man',name:'深蓝休闲 · 男模特',description:'自然生活方式',image:'/model-gallery/02-black-man.jpg'},
 {id:'03-south-asian-woman',name:'橄榄通勤 · 女模特',description:'清爽通勤',image:'/model-gallery/03-south-asian-woman.jpg'},
 {id:'04-east-asian-man',name:'蓝衬衫讲解 · 男模特',description:'专业亲和',image:'/model-gallery/04-east-asian-man.jpg'},
 {id:'05-latina-woman',name:'砖红生活 · 女模特',description:'成熟自然',image:'/model-gallery/05-latina-woman.jpg'},
 {id:'06-middle-eastern-man',name:'浅沙外套 · 男模特',description:'都市休闲',image:'/model-gallery/06-middle-eastern-man.jpg'},
 {id:'07-active-yoga-woman',name:'自然瑜伽 · 女模特',description:'原创 AI · 轻运动体态 · 全身棚拍',image:'/model-gallery/07-active-yoga-woman.png'},
 {id:'08-active-training-woman',name:'力量训练 · 女模特',description:'原创 AI · 自然力量感 · 全身棚拍',image:'/model-gallery/08-active-training-woman.png'},
 {id:'09-active-commute-man',name:'运动通勤 · 男模特',description:'原创 AI · 健身型体态 · 全身棚拍',image:'/model-gallery/09-active-commute-man.png'},
];
export default function CharacterLibrary({ initialTab='library', onPick, onClose }) {
 const dialog=useRef(null), upload=useRef(null);
 const [tab,setTab]=useState(initialTab),[roles,setRoles]=useState([]),[reload,setReload]=useState(0);
 const [loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[galleryBusy,setGalleryBusy]=useState(''),[error,setError]=useState('');
 const [imageOptions,setImageOptions]=useState({});
 const [name,setName]=useState('新角色'),[controls,setControls]=useState(defaults),[reference,setReference]=useState(null),[preview,setPreview]=useState(null);
 const [file,setFile]=useState(null),[fileURL,setFileURL]=useState(''),[query,setQuery]=useState('');
 useEffect(()=>{dialog.current.showModal();},[]);
 useEffect(()=>{if(!file){setFileURL('');return;}const url=URL.createObjectURL(file);setFileURL(url);return()=>URL.revokeObjectURL(url);},[file]);
 useEffect(()=>{
  const controller=new AbortController();let timer;
  async function refresh(){try{const next=await listCharacters(controller.signal);if(controller.signal.aborted)return;setRoles(next);setLoading(false);if(next.some(r=>['upload','ai','edit'].includes(r.source)&&['generating','queued'].includes(r.status)))timer=setTimeout(refresh,2500);}catch(e){if(!controller.signal.aborted){setError(e.message);setLoading(false);}}}
  refresh();return()=>{controller.abort();clearTimeout(timer);};
 },[reload]);
 const characterRoles=roles.filter(r=>['upload','ai','edit'].includes(r.source));
 const ready=characterRoles.filter(r=>r.status==='ready');
 const pending=characterRoles.filter(r=>r.status!=='ready');
 const visible=(tab==='history'?pending:ready).filter(r=>r.name.toLowerCase().includes(query.trim().toLowerCase()));
 const visibleGallery=gallery.filter(model=>`${model.name} ${model.description}`.toLowerCase().includes(query.trim().toLowerCase()));
 const selected=roles.find(r=>r.id===preview);
 const base=selected?.reference_id ? roles.find(r=>r.id===selected.reference_id) : null;
 function field(key,value){setControls(c=>({...c,[key]:value}));}
 function changeLook(role){setReference(role);setControls({...defaults,...role.controls});setName(`${role.name} · 新造型`.slice(0,80));setTab('generate');setError('');setPreview(null);}
 async function create(){setBusy(true);setError('');try{const role=await generateCharacter({name,reference_id:reference?.id||'',controls,image_options:imageOptions});setTab(role.status==='ready'?'library':'history');setQuery('');setPreview(role.id);setReload(x=>x+1);}catch(e){setError(e.message);}finally{setBusy(false);}}
 async function saveUpload(){if(!file)return;setBusy(true);setError('');try{const role=await uploadCharacter(name,file);setFile(null);setTab(role.status==='ready'?'library':'history');setQuery('');setPreview(role.id);setReload(x=>x+1);}catch(e){setError(e.message);}finally{setBusy(false);}}
 async function useGalleryModel(model){
  setGalleryBusy(model.id);setError('');
  try{
   const response=await fetch(model.image,{credentials:'same-origin'});
   const mime=response.headers.get('content-type')?.split(';')[0];
   if(!response.ok||!['image/jpeg','image/png'].includes(mime))throw new Error('模特图片暂时不可用，请刷新后重试');
   const image=await response.blob();
   const role=await uploadCharacter(model.name,new File([image],`${model.id}.${mime==='image/png'?'png':'jpg'}`,{type:mime}));
   setRoles(current=>[role,...current]);
   onPick({id:role.id,character_id:role.id,name:role.name,kind:'image'});
  }catch(e){setError(e.message);}finally{setGalleryBusy('');}
 }
 function pick(role){onPick({id:role.id,character_id:role.id,name:role.name,kind:'image'});}
 return <dialog ref={dialog} className="character-dialog" aria-labelledby="character-library-title" onCancel={onClose}>
  <header><div><h2 id="character-library-title">角色库</h2><p>选择现成模特，或使用自己保存的角色，在不同版本中复用。</p></div><button className="tb-icon-button" aria-label="关闭角色库" onClick={onClose}><X size={20}/></button></header>
  <nav className="character-tabs" aria-label="角色来源">{[['library','我的角色'],['generate','AI 生成角色'],['upload','上传角色'],['history',`生成记录${pending.length ? ` · ${pending.length}` : ''}`]].map(([id,label])=><button key={id} aria-pressed={tab===id} disabled={busy} onClick={()=>{setTab(id);setError('');}}>{label}</button>)}</nav>
  <div className="character-body">
   {error && <div className="tb-inline-note" role="alert">{error}<button className="tb-text-button" onClick={()=>{setError('');setReload(x=>x+1);}}>重新读取角色库</button></div>}
   {['library','history'].includes(tab) && <>
    <label className="tb-field">查找角色或模特<input value={query} onChange={e=>setQuery(e.target.value)} placeholder="搜索名称或风格"/></label>
    {tab==='library' && !!visibleGallery.length && <section className="character-gallery" aria-labelledby="character-gallery-title"><div className="character-section-heading"><div><h3 id="character-gallery-title">精选模特图</h3><p>{gallery.length} 张原创 AI 模特图，无需调用模型；选用后保存到你的角色库。</p></div></div><div className="character-grid">{visibleGallery.map(model=><article key={model.id}><img className="character-gallery-image" src={model.image} alt={model.name} loading="lazy"/><strong className="character-gallery-name">{model.name}</strong><small>{model.description}</small><div className="character-card-actions"><button className="tb-secondary" disabled={!!galleryBusy||busy} onClick={()=>useGalleryModel(model)}>{galleryBusy===model.id?<><Loader2 className="spin" size={14}/>正在保存…</>:'使用此模特'}</button></div></article>)}</div></section>}
    <section className="character-owned" aria-labelledby="character-owned-title"><div className="character-section-heading"><div><h3 id="character-owned-title">{tab==='history'?'角色生成记录':'我的角色'}</h3><p>{tab==='history'?'仅展示角色的进行中及失败记录；姿势作品请在画布作品库查看。':'仅展示已成功生成或上传的角色。'}</p></div></div>
    {loading ? <p role="status">正在读取角色…</p> : !visible.length ? <div className="character-empty"><p>{tab==='history'?'暂无进行中或失败的角色生成记录。':'没有匹配的可用角色。可以选用精选模特，或上传和生成自己的角色。'}</p></div> : <div className="character-grid">{visible.map(role=><article key={role.id} className={preview===role.id?'selected':''}>
     {role.status==='ready'?<DownloadableImage src={characterImage(role)} alt={role.name} filename={role.name} className="character-owned-image" previewClassName="character-preview-button" onPreview={()=>setPreview(role.id)}/>:<div className="character-pending">{role.status==='generating'?<><Loader2 className="spin"/>生成中</>:'未生成成功'}</div>}<strong className="character-gallery-name">{role.name}</strong>
     <small>{role.source==='upload'?'上传角色':role.source==='edit'?'角色新造型':'AI 角色'}</small>
     {role.status==='ready'?<div className="character-card-actions"><button className="tb-secondary" onClick={()=>pick(role)}>使用</button><button className="tb-text-button" onClick={()=>changeLook(role)}>改造型</button></div>:role.status==='failed'?<p role="alert">{role.error}</p>:<p>关闭窗口后仍会继续生成</p>}
    </article>)}</div>}
    </section>
    {selected?.status==='ready' && <section className="character-comparison" aria-label="角色预览">{base?.status==='ready'&&<figure><DownloadableImage src={characterImage(base)} alt="原角色" filename={base.name}/><figcaption>基准角色</figcaption></figure>}<figure><DownloadableImage src={characterImage(selected)} alt={selected.name} filename={selected.name}/><figcaption>{selected.name}</figcaption></figure><div><p>{base?'检查五官、肤色与发型是否延续原角色，再采用新造型。':'确认角色外观后，加入当前换模特版本。'}</p><button className="tb-primary" onClick={()=>pick(selected)}>使用此角色</button></div></section>}
   </>}
   {tab==='upload'&&<div className="character-upload"><label className="tb-field">角色名称<input value={name} maxLength={80} onChange={e=>setName(e.target.value)}/></label><input ref={upload} type="file" hidden accept=".jpg,.jpeg,.png" onChange={e=>{setFile(e.target.files?.[0]||null);setError('');}}/><button className="tb-dropzone" onClick={()=>upload.current?.click()}><Upload size={24}/><strong>{file?file.name:'选择角色图'}</strong><small>PNG / JPG · 最大 10MB · 宽高 384–3072 像素</small></button>{fileURL&&<img className="character-upload-preview" src={fileURL} alt="待保存角色"/>}</div>}
   {tab==='generate'&&<>
    <label className="tb-field">角色名称<input maxLength={80} value={name} onChange={e=>setName(e.target.value)}/></label>
    <label className="tb-field">身份基准<select value={reference?.id||''} onChange={e=>{const role=ready.find(r=>r.id===e.target.value);setReference(role||null);}}><option value="">创建一个新角色</option>{ready.map(r=><option key={r.id} value={r.id}>沿用：{r.name}</option>)}</select></label>
    {reference?<div className="character-reference"><img src={characterImage(reference)} alt="身份基准"/><div><strong>以「{reference.name}」为基准</strong><p>保持五官、肤色、年龄与发型，仅调整下面的造型。生成后需对比确认。</p></div></div>:<>
     <div className="character-presets" aria-label="角色设定模板">{presets.map(p=><button key={p.name} onClick={()=>{setName(p.name);const {name: presetName,...values}=p;setControls({...defaults,...values});}}>{p.name}</button>)}</div><p className="tb-muted">模板只提供创作设定，点击生成后才会创建角色图。</p>
     <label className="tb-field">外观描述<textarea rows={2} maxLength={200} value={controls.description} onChange={e=>field('description',e.target.value)}/></label>
     <div className="character-fields"><label className="tb-field">年龄<input type="number" min={18} max={90} value={controls.age} onChange={e=>field('age',Number(e.target.value))}/></label><label className="tb-field">发型<input maxLength={200} value={controls.hair} onChange={e=>field('hair',e.target.value)}/></label></div>
    </>}
    <div className="character-fields">{[['outfit','服装'],['expression','表情'],['pose','姿态'],['framing','构图'],['background','背景'],['lighting','光线']].map(([key,label])=><label className="tb-field" key={key}>{label}<input maxLength={200} value={controls[key]} onChange={e=>field(key,e.target.value)}/></label>)}</div>
    {!imageOptions.model&&<details className="tb-details"><summary>更多控制<ChevronDown size={16}/></summary><div className="character-fields"><label className="tb-field">图片尺寸<select value={controls.size} onChange={e=>field('size',e.target.value)}><option value="720*1280">9:16 · 720 × 1280</option><option value="960*1280">3:4 · 960 × 1280</option><option value="1024*1024">1:1 · 1024 × 1024</option></select></label><label className="tb-field">种子<input type="number" min={0} max={2147483647} value={controls.seed} onChange={e=>field('seed',Number(e.target.value))}/></label></div><button className="tb-text-button" onClick={()=>field('seed',Math.floor(Math.random()*2147483648))}>换一个种子</button><p className="tb-muted">相同种子有助于稳定结果，不能代替身份参考图。</p></details>}
    <label className="tb-field">避免事项<input maxLength={200} value={controls.negative} onChange={e=>field('negative',e.target.value)} placeholder="例如：帽子、墨镜、夸张妆容"/></label><p className="tb-muted">避免事项作为文字描述加入提示词；服装与姿态仍需人工核对。</p>

   </>}
  </div>
  {tab==='generate'&&<ImageModelPicker value={imageOptions} onChange={setImageOptions} defaultRatio="9:16" disabled={busy} capability={reference?"image_edit":"image_generation"} />}
  {['generate','upload'].includes(tab)&&<footer><small>{tab==='generate'?'一次生成 1 张，使用已配置的图片模型。':'保存在当前账号的角色库中。'}</small><button className="tb-primary" disabled={busy||!name.trim()||(tab==='upload'&&!file)} onClick={tab==='upload'?saveUpload:create}>{busy?<Loader2 className="spin" size={16}/>:tab==='generate'?<Sparkles size={16}/>:<Plus size={16}/>}{busy?'正在提交…':tab==='upload'?'保存角色':reference?'生成新造型':'生成角色图'}</button></footer>}
 </dialog>;
}
