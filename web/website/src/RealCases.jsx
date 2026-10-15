import { ArrowRight, ArrowUpRight, ArrowsOut } from '@phosphor-icons/react';
import './RealCases.css';

const caseAssets = 'assets/cases/';

function CaseImage({ file, alt, label, className = '' }) {
  return <figure className={`case-image ${className}`.trim()}>
    <a href={`${caseAssets}${file}`} target="_blank" rel="noopener noreferrer" aria-label={`${label} · ${alt}`}>
      <img src={`${caseAssets}${file}`} alt={alt} loading="lazy" decoding="async" />
      <span className="case-image-expand" aria-hidden="true"><ArrowsOut size={18} /></span>
    </a>
  </figure>;
}

export default function RealCases({ lang }) {
  const zh = lang === 'zh';
  const labels = zh ? ['正面站姿', '侧身站姿', '抬手展示', '自然站姿'] : ['Front pose', 'Side pose', 'Raised arm', 'Relaxed stance'];
  const enlarge = zh ? '放大查看' : 'View full image';
  return <section id="cases" className="section real-cases">
    <div className="intro" data-reveal>
      <span className="eyebrow"><i />REAL WORK / {zh ? '实际案例' : 'ACTUAL RESULTS'}</span>
      <h2>{zh ? '先看作品，再聊可能。' : 'See what comes out.'}</h2>
      <p>{zh ? '来自实际生成任务，图片未经修图。' : 'From actual generation tasks, without retouching.'}</p>
    </div>
    <article className="real-case" data-reveal>
      <header className="real-case-heading">
        <div><span className="case-number">01 / MODEL POSES</span><h3>{zh ? '一件外套，四个姿势。' : 'One jacket. Four poses.'}</h3></div>
        <a className="text-link" href="examples/apparel-images/">{zh ? '看看怎么做' : 'See the steps'}<ArrowUpRight /></a>
      </header>
      <div className="case-pose-layout">
        <div className="case-input"><CaseImage file="jacket-input.png" alt={zh ? '黑色连帽短外套商品参考图' : 'Black hooded jacket product reference'} label={enlarge} /><span>{zh ? '商品参考' : 'Product reference'}</span></div>
        <ArrowRight className="case-flow-arrow" aria-hidden="true" />
        <div className="case-poses">{labels.map((label, index) => <div key={index}><CaseImage file={`pose-${index + 1}.png`} alt={zh ? `实际生成的黑色外套模特图：${label}` : `Actual generated jacket model image: ${label}`} label={enlarge} /><span>{label}</span></div>)}</div>
      </div>
      <p className="case-caption">{zh ? '商品、模特与动作分别使用对应参考。选几个姿势，就生成几张图。' : 'Separate references for the product, model and pose. One output per selected pose.'}</p>
    </article>
    <article className="real-case" data-reveal>
      <header className="real-case-heading">
        <div><span className="case-number">02 / REFERENCE RECREATION <b>{zh ? '开发预览' : 'Development preview'}</b></span><h3>{zh ? '喜欢这个构图？换上自己的商品。' : 'Keep the layout. Make it yours.'}</h3></div>
        <a className="text-link" href="examples/reference-recreation/">{zh ? '查看完整案例' : 'View the case'}<ArrowUpRight /></a>
      </header>
      <div className="case-comparison">
        <div><CaseImage file="reference-original.jpg" alt={zh ? '原始三栏服装展示参考画面' : 'Original three-column apparel reference'} label={enlarge} /><span>{zh ? '参考画面' : 'Reference'}</span></div>
        <div><CaseImage file="reference-result.png" alt={zh ? '实际生成结果：相同构图，替换为黑色短外套与指定模特' : 'Actual result with the replacement black jacket and model'} label={enlarge} /><span>{zh ? '替换商品与模特后的结果' : 'New product and model'}</span></div>
      </div>
      <p className="case-caption">{zh ? '此场景尚未包含在当前公开版本中。画面标识来自参考素材，不代表品牌合作。' : 'This scene is not yet included in the public release. Marks originate from the reference; no brand partnership is implied.'}</p>
      <details className="case-workflow"><summary>{zh ? '查看实际工作流' : 'View the actual workflow'}</summary><CaseImage file="reference-workflow.png" alt={zh ? '参考图复刻实际工作流截图，已隐去文件名与计费信息' : 'Actual reference recreation workflow; filenames and billing hidden'} label={enlarge} /></details>
    </article>
  </section>;
}
