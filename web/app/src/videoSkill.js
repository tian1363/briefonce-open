export function prepareVideoDraft(value = '', context = '') {
  const text = value.trim();
  const explicitDuration = source => source.match(/(?:脚本总时长|总时长|视频时长|成片时长|duration)(?:\*\*)?\s*[:：=]?\s*(?:\*\*)?\s*(\d+)\s*(?:\*\*)?\s*(?:秒|s)?/i) || source.match(/(?:^|\n)\s*(?:[-*]\s*)?(?:时长)\s*[:：=]\s*(\d+)\s*(?:秒|s)?/i);
  const durationMatch = explicitDuration(text) || context.split('\n').reverse().map(explicitDuration).find(Boolean);
  const sceneDuration = [...text.matchAll(/(?:分镜|镜头)\s*\d+[^\n]*?[（(]\s*\d{1,2}:\d{2}\s*[-–—~至]\s*(\d{1,2}):(\d{2})\s*[）)]/g)]
    .reduce((max, match) => Math.max(max, Number(match[1]) * 60 + Number(match[2])), 0);
  const fallbackDuration = text.match(/(\d+)\s*(?:秒|seconds?\b|s\b)/i)?.[1];
  let duration = Math.max(2, Math.min(30, Number(durationMatch?.[1] || sceneDuration || fallbackDuration || 5)));
  const ratio = text.match(/(?:16\s*[:：]\s*9|9\s*[:：]\s*16|1\s*[:：]\s*1)/)?.[0].replace(/\s/g, '').replace('：', ':') || (/横屏/.test(text) ? '16:9' : /方形/.test(text) ? '1:1' : '9:16');
  const resolution = text.match(/\b(480|720|1080)p\b/i)?.[0].toUpperCase() || '720P';
  let soundEnabled = !/(?:^|\n)\s*(?:[-*]\s*)?(?:\*\*)?(?:声音|音频|生成声音|audio|sound_enabled)(?:\*\*)?\s*[:：=]\s*(?:静音|无声|关闭|不生成|false|off|0)(?=\s|$|[，,。；;])/im.test(text)
    && !/(?:^|\n)\s*(?:请)?(?:关闭声音|不生成声音|生成(?:一个|一条|一段)?(?:无声|静音)视频|(?:整条|整个|成片)视频(?:设为|设置为|改为)?(?:静音|无声))(?=$|[\s。，,；;])/u.test(text);
  const section = text.match(/(?:^|\n)\s*(?:#{1,6}\s*)?(?:\*\*)?(?:(?:Seedance|即梦)\s*)?(?:视频生成提示词|视频提示词|生成提示词|Video Prompt|Prompt)(?:\*\*)?\s*[:：]?[^\S\n]*\n?([\s\S]*)/i);
  let prompt = section ? section[1] : text;
  if (!section) {
    const storyboard = storyboardVideoPrompt(text);
    if (storyboard) {
      prompt = storyboard.prompt;
      duration = storyboard.duration ? Math.max(duration, storyboard.duration) : duration;
      if (storyboard.silent) soundEnabled = false;
    } else if (/(?:分镜|脚本|素材清单|合规声明)/.test(text) && /\|[^\n]+\|/.test(text)) {
      prompt = '';
    }
  }
  prompt = prompt.split(/\n\s*(?:#{1,6}\s*)?(?:\*\*)?(?:说明|解释|注意事项|使用建议|参数设置|生成参数|视频参数|下一步|为什么)(?:\*\*)?\s*[:：\n]/)[0];
  const fenced = prompt.match(/```(?:text|plaintext|prompt)?\s*\n([\s\S]*?)```/i);
  if (fenced) prompt = fenced[1];
  if (/###\s*分镜\s*\d+[\s\S]*?(?:\*\*)?正向提示词/.test(prompt)) {
    const firstPositive = prompt.search(/(?:\*\*)?正向提示词(?:\*\*)?\s*[:：]/);
    if (firstPositive >= 0) prompt = prompt.slice(firstPositive).replace(/^(?:\*\*)?正向提示词(?:\*\*)?\s*[:：](?:\*\*)?\s*/, '');
    prompt = prompt.split(/\n\s*(?:✅\s*)?(?:\*\*)?(?:使用说明|说明|下一步)(?:\*\*)?\s*[:：]/)[0];
  }
  prompt = prompt.split('\n').filter(line => !/^\s*(?:[-*]\s*)?(?:时长|总时长|分辨率|画幅|比例|声音|duration|resolution|ratio|audio)\s*[:：=]/i.test(line.replace(/\*\*/g, '')) && !/^\s*(?:好的[，,！!。]|以下是|下面是|你可以|如果你|如需|希望这|点击.*生成)/.test(line)).join('\n').trim();
  prompt = prompt.replace(/(?:\n\s*---\s*)+$/, '').trim();
  return { prompt, duration, ratio, resolution, soundEnabled };
}

function storyboardVideoPrompt(text) {
  const lines = text.split('\n');
  const headerIndex = lines.findIndex(line => /^\s*\|/.test(line) && /时间/.test(line) && /画面|镜头|分镜/.test(line));
  if (headerIndex < 0) return null;
  const cells = line => line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map(cell => cell.trim().replace(/<br\s*\/?\s*>/gi, '，').replace(/\*\*/g, ''));
  const headers = cells(lines[headerIndex]);
  const index = pattern => headers.findIndex(header => pattern.test(header));
  const timeIndex = index(/时间/);
  const visualIndex = index(/画面|镜头|分镜/);
  const voiceIndex = index(/口播|旁白|台词/);
  const subtitleIndex = index(/字幕|屏幕文字|文案/);
  const soundIndex = index(/声音|音效|环境声/);
  const musicIndex = index(/BGM|背景音乐/i);
  const roleIndex = index(/人物|角色/);
  const sceneIndex = index(/场景|环境/);
  const actionIndex = index(/动作|互动/);
  const propIndex = index(/道具/);
  const cameraIndex = index(/景别|机位|镜头运动/);
  const movementIndex = index(/镜头运动|转场/);
  const rhythmIndex = index(/节奏|停顿/);
  if (timeIndex < 0 || visualIndex < 0) return null;
  const scenes = [];
  let lastEnd = 0;
  let allSilent = voiceIndex >= 0 && soundIndex >= 0 && musicIndex >= 0;
  for (const line of lines.slice(headerIndex + 1)) {
    if (!/^\s*\|/.test(line)) break;
    if (/^\s*\|[\s:|-]+\|?\s*$/.test(line)) continue;
    const row = cells(line);
    const time = row[timeIndex] || '';
    const visual = row[visualIndex] || '';
    if (!visual || !/\d/.test(time)) continue;
    if (allSilent && ![voiceIndex, soundIndex, musicIndex].every(column => /^无$/.test(row[column] || ''))) allSilent = false;
    const parts = [`${time}：${visual}`];
    const usedColumns = new Set([visualIndex]);
    for (const [column, label] of [[roleIndex, '人物角色'], [sceneIndex, '场景环境'], [actionIndex, '动作互动'], [propIndex, '道具'], [cameraIndex, '镜头'], [movementIndex, '运动转场'], [rhythmIndex, '剪辑节奏']]) {
      if (column >= 0 && !usedColumns.has(column) && row[column] && !/^[-—无]$/.test(row[column])) parts.push(`${label}：${row[column]}`);
      usedColumns.add(column);
    }
    if (voiceIndex >= 0 && row[voiceIndex] && !/^[-—]$/.test(row[voiceIndex])) parts.push(`口播：${row[voiceIndex]}`);
    if (subtitleIndex >= 0 && row[subtitleIndex] && !/^[-—无]$/.test(row[subtitleIndex])) parts.push(`字幕：${row[subtitleIndex]}`);
    if (soundIndex >= 0 && row[soundIndex] && !/^[-—]$/.test(row[soundIndex])) parts.push(`音效与环境声：${row[soundIndex]}`);
    if (musicIndex >= 0 && row[musicIndex] && !/^[-—]$/.test(row[musicIndex])) parts.push(`BGM：${row[musicIndex]}`);
    scenes.push(parts.join('；'));
    const lastTime = time.match(/(\d{1,2}:\d{2}|\d+(?:\.\d+)?)(?!.*\d)/)?.[1];
    if (lastTime) {
      const end = lastTime.includes(':') ? Number(lastTime.split(':')[0]) * 60 + Number(lastTime.split(':')[1]) : Number(lastTime);
      lastEnd = Math.max(lastEnd, end);
    }
  }
  if (!scenes.length) return null;
  return {prompt: scenes.join('\n'), duration: lastEnd ? Math.max(2, Math.min(30, Math.ceil(lastEnd))) : null, silent: allSilent};
}

export function hasVideoScript(value = '') {
  return /(?:^|\n)\s*(?:#{1,6}\s*)?(?:(?:Seedance|即梦)\s*)?(?:视频生成提示词|视频提示词)\s*[:：\n]/i.test(value) || /\|[^\n]*时间[^\n]*\|[^\n]*(?:画面|镜头|分镜)/.test(value);
}

export function videoDraftContext(messages = []) {
  const latestScript = messages.findLastIndex(message => message.role === 'assistant' && hasVideoScript(message.content || ''));
  const start = latestScript < 0 ? Math.max(0, messages.length - 6) : Math.max(0, latestScript - 4);
  return messages.slice(start, start + 10).map(message => message.content || '').join('\n');
}

export function videoReferenceIDs(value = '') {
  const references = [...value.matchAll(/\/api\/assets\/([a-zA-Z0-9_-]+)\/file|\bCID\s*[:：]?\s*\*{0,2}asset-([a-zA-Z0-9_-]+)\*{0,2}/gi)];
  return [...new Set(references.map(match => match[1] || match[2]))];
}

export function parseVideoSkillRequest(value = "") {
  const text = value.trim();
  // A request for prompt text belongs in chat, even when it names the video skill.
  if (!/^\/(?:生成视频|generate-video)(?=$|[\s，,:：。])/u.test(text) && /(?:提示词|prompt|教程|方案|用法|说明)/i.test(text)) return null;
  const explicit = text.match(/^(?:\/(?:生成视频|generate-video)|(?:调用|使用)\s*`?(?:generate-video|generate_video|生成视频)`?\s*(?:skill|技能)?)(?=$|[\s，,:：。])[\s，,:：。]*(.*)$/isu);
  if (explicit) return { prompt: explicit[1].trim() };
  if (/不要|别生成|不生成|怎么|如何|能否|能不能|可以吗|能做什么|是什么/.test(text)) return null;
  if (/^(?:请)?(?:帮我|给我)?(?:直接|立即|开始)?生成(?:完整|全部|整段|整条)(?:的)?视频[。！!]?$/u.test(text)) return { prompt: '' };
  const previous = /^(?:请)?(?:把|将|用|按|基于)?(?:上面|上文|刚才|这段|这个)(?:的)?(?:脚本|内容|提示词)?(?:直接)?(?:生成|制作|做成)(?:一个|一段)?视频[。！!]?$/u;
  if (previous.test(text) || /^(?:请)?(?:直接|立即|开始)?(?:给我|帮我)?生成(?:这个|这段|一段|一个)?视频[。！!]?$/u.test(text)) return { prompt: "" };
  if (/^(?:请)?(?:帮我|给我)?(?:生成|制作)(?:一条|一段|一个)?.{0,120}视频/su.test(text) && !/提示词|教程|方案/.test(text)) return { prompt: text };
  return null;
}
