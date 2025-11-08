import { Camera, FileText, Globe2, Workflow } from 'lucide-react';
import { WorkTypeCard } from './components/WorkTypeCard';

const WORK_TYPES = [
  {
    slug: 'image',
    title: '图片创作',
    description: '与 Agent 协作设计灵感图、海报、品牌物料。',
    highlights: ['描述灵感、快速生成初稿', '自动整理素材与颜色板', '结合 crafts 管理视觉产物'],
    icon: <Camera className="h-6 w-6" aria-hidden />,
  },
  {
    slug: 'article',
    title: '文章创作',
    description: '所见即所得的写作环境，随时引用 Agent 给出的资料。',
    highlights: ['文稿实时编辑与格式化', '自动整理参考资料', '嵌入沙箱动作追踪'],
    icon: <FileText className="h-6 w-6" aria-hidden />,
  },
  {
    slug: 'web',
    title: '网页搭建',
    description: '快速拼装或改造网页原型，自动生成可复用组件。',
    highlights: ['HTML/CSS 草稿', '联动代码微服务部署', '输出网页产物到 crafts'],
    icon: <Globe2 className="h-6 w-6" aria-hidden />,
  },
  {
    slug: 'code',
    title: '代码微服务 Demo',
    description: '以任务驱动的脚手架，快速验证服务端/前端 demo。',
    highlights: ['自动初始化项目骨架', '在沙箱中运行与调试', '连接 API 流程与文档'],
    icon: <Workflow className="h-6 w-6" aria-hidden />,
  },
] as const;

export default function WorkbenchPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-slate-950 via-slate-900 to-slate-950 py-16">
      <div className="mx-auto max-w-6xl px-6">
        <header className="mb-12 text-center">
          <p className="text-sm font-medium text-cyan-300/80">Alex Workbench</p>
          <h1 className="mt-4 text-4xl font-semibold text-slate-50">选择今天要完成的目标</h1>
          <p className="mt-3 text-base text-slate-400">
            针对不同产物类型提供专属环境，Agent 会在整个流程中提供资料、执行与产物整理。
          </p>
        </header>
        <div className="grid gap-6 md:grid-cols-2">
          {WORK_TYPES.map((type) => (
            <WorkTypeCard
              key={type.slug}
              href={`/workbench/${type.slug}`}
              title={type.title}
              description={type.description}
              highlights={type.highlights}
              icon={type.icon}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
