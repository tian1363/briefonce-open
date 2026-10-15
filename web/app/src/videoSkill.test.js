import test from 'node:test';
import assert from 'node:assert/strict';
import {parseVideoSkillRequest} from './videoSkill.js';
import {hasVideoScript, prepareVideoDraft, videoDraftContext, videoReferenceIDs} from './videoSkill.js';
test('extracts only prompt body and fills all requested parameters',()=>{
 const result = prepareVideoDraft('好的，以下是提示词。\n## 视频生成提示词\n```text\n海边日落，模特拿起产品。\n镜头缓慢推进。\n```\n## 视频参数\n时长：7秒\n画幅：16:9\n分辨率：1080P\n声音：静音\n## 说明\n你可以复制使用。');
 assert.deepEqual(result,{prompt:'海边日落，模特拿起产品。\n镜头缓慢推进。',duration:7,ratio:'16:9',resolution:'1080P',soundEnabled:false});
});
test('preserves plain scene copy and provides valid defaults',()=>{
 assert.deepEqual(prepareVideoDraft('人物走进厨房，拿起咖啡。'),{prompt:'人物走进厨房，拿起咖啡。',duration:5,ratio:'9:16',resolution:'720P',soundEnabled:true});
 assert.equal(prepareVideoDraft('时长：60秒\n画幅：横屏\n一个产品特写').duration,30);
 assert.equal(prepareVideoDraft('脚本总时长 **8**。\n画面从清晨开始。').duration,8);
});
test('scene level silence does not turn off sound for the whole video',()=>{
 const script = '视频生成提示词\n街头采访，保留人物对白与环境音。\nBGM：前段高潮后突然静音，制造悬念。';
 assert.equal(prepareVideoDraft(script).soundEnabled,true);
 assert.equal(prepareVideoDraft(script+'\n视频参数\n声音：静音').soundEnabled,false);
 assert.equal(prepareVideoDraft('请生成一条无声视频').soundEnabled,false);
});
test('uses Seedance scene timing and starts at the first positive prompt',()=>{
 const source = '以下是基于分镜脚本生成的提示词。\n\n---\n# Seedance 视频生成提示词\n## 脚本名称：《一天三变》\n### 全片连续性要求：\n- 同一位模特\n---\n### 分镜 01（00:00-00:02）\n**正向提示词：**  \n模特走出地铁站。\n**声音与字幕：**\n脚步声\n### 分镜 02（00:03-00:05）\n**正向提示词：**\n模特坐在窗边。\n### 分镜 03（00:06-00:08）\n**正向提示词：**\n模特走入公园。\n✅ **使用说明：**\n请先测试。';
 const result = prepareVideoDraft(source);
 assert.equal(result.duration, 8);
 assert.equal(result.resolution, '720P');
 assert.match(result.prompt, /^模特走出地铁站。/);
 assert.match(result.prompt, /模特走入公园。/);
 assert.doesNotMatch(result.prompt, /脚本名称|全片连续性要求|使用说明|以下是/);
 assert.equal(prepareVideoDraft(source + '\n视频参数\n分辨率：1080P').resolution, '1080P');
});
test('fills total duration from nearby script context',()=>{
 const messages = [
  {role:'user',content:'脚本总时长 **6**。'},
  {role:'assistant',content:'# Seedance 视频生成提示词\n### 分镜 01（00:00-00:02）\n**正向提示词：**\n模特出场。'},
  {role:'user',content:'脚本总时长：8秒。'},
 ];
 assert.equal(hasVideoScript(messages[1].content),true);
 assert.equal(prepareVideoDraft(messages[1].content,videoDraftContext(messages)).duration,8);
});
test('does not send a storyboard table as a video prompt',()=>{
 const result = prepareVideoDraft('## 15秒脚本\n| 时间 | 分镜 |\n|---|---|\n| 00:00-00:03 | 主持人提问 |\n## 合规声明\n仅供拍摄参考');
 assert.equal(result.prompt, '00:00-00:03：主持人提问');
 assert.equal(result.duration, 15);
});
test('bridges a storyboard to an editable video draft without asset IDs or follow-up advice',()=>{
 const source = '🎬【完整分镜表】\n| 时间 | 画面与动作 | 口播/旁白 | 字幕文案 | 声音/剪辑 | 所需素材 (CID) |\n|---|---|---|---|---|---|\n| 0-3s | 主持人走入画面 | “今天聊什么？” | 街头提问 | 跟拍，轻快音乐 | /api/assets/secret/file |\n| 3-15s | 产品特写 | “这是什么？” | 产品名称 | 硬切 | 待补拍 |\n\n📤 下一步：调用 generate-video skill 输出提示词。';
 const result = prepareVideoDraft(source);
 assert.equal(result.duration,15);
 assert.match(result.prompt,/0-3s：主持人走入画面；口播：/);
 assert.match(result.prompt,/3-15s：产品特写/);
 assert.doesNotMatch(result.prompt,/\/api\/assets|下一步|待补拍|CID/);
 assert.equal(hasVideoScript(source),true);
 assert.equal(hasVideoScript('我已经完成多轮工具检查，但没有答案。'),false);
 assert.deepEqual(videoReferenceIDs(source),['secret']);
 assert.deepEqual(videoReferenceIDs('主镜头：**CID asset-job_6af7935c566d028f**；尾版 CID asset-job_0a87741f5e54a891。'),['job_6af7935c566d028f','job_0a87741f5e54a891']);
});
test('carries scene interactions, camera rhythm and verified silence into the video draft',()=>{
 const source = '| 时间段 | 画面描述 | 人物与角色 | 场景与环境 | 动作与环境互动 | 道具 | 屏幕文字/文案 | 旁白/对话 | 景别与机位 | 镜头运动/转场 | 叙事节奏/停顿 | 音效/环境声 | BGM |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|\n| 00:00-00:03 | 主持人进入画面 | 主持人 | 街头 | 与路人交谈 | 麦克风 | 你会选谁 | 无 | 中景 | 跟拍 | 结尾停顿 | 无 | 无 |';
 const result = prepareVideoDraft(source);
 assert.match(result.prompt,/场景环境：街头/);
 assert.match(result.prompt,/动作互动：与路人交谈/);
 assert.match(result.prompt,/剪辑节奏：结尾停顿/);
 assert.match(result.prompt,/口播：无/);
 assert.match(result.prompt,/BGM：无/);
 assert.equal(result.soundEnabled,false);
});
test('video skill triggers explicit commands and conversational video requests',()=>{
 for(const text of ['生成完整视频','生成全部视频','请帮我生成完整视频']) assert.deepEqual(parseVideoSkillRequest(text),{prompt:''});
 for(const text of ['/生成视频','/generate-video','调用 generate-video skill','调用 `generate-video` skill','把上面的脚本生成视频','按上文生成视频','生成视频']) assert.deepEqual(parseVideoSkillRequest(text),{prompt:''});
 assert.deepEqual(parseVideoSkillRequest('/生成视频 日落海边，5秒'),{prompt:'日落海边，5秒'});
 assert.deepEqual(parseVideoSkillRequest('帮我生成一个北美模特的视频'),{prompt:'帮我生成一个北美模特的视频'});
});
test('questions, negation and prompt writing do not submit video requests',()=>{
 for(const text of ['怎么生成视频','不要生成视频','生成一个视频提示词','调用 `generate-video` skill 输出AI视频生成提示词','调用 generate-video skill 输出视频提示词','帮我写视频方案','这个技能有什么用','/generate-video-prompt']) assert.equal(parseVideoSkillRequest(text),null,text);
});
