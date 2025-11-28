import Link from "next/link";

import {
  PageContainer,
  PageShell,
  ResponsiveGrid,
  SectionBlock,
  SectionHeader,
} from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

const featureCards = [
  {
    title: "实时控制台",
    description: "对话、工具输出与事件同步呈现，像产品控制台一样可视。",
    action: { label: "进入会话", href: "/conversation" },
  },
  {
    title: "团队入口",
    description: "登录即可共享历史与预设，保持团队上下文一致。",
    action: { label: "登录", href: "/login" },
  },
  {
    title: "清爽布局",
    description: "为宽屏与移动端优化的响应式骨架，无多余装饰。",
    action: { label: "浏览历史", href: "/sessions" },
  },
];

const workflow = [
  {
    title: "开启会话",
    description: "从登录或历史记录进入，快速拉起上下文。",
  },
  {
    title: "协作处理",
    description: "对话流、工具回调与事件时间线同屏协同。",
  },
  {
    title: "复盘交付",
    description: "归档会话并再次访问，保持链路透明可追溯。",
  },
];

const quickLinks = [
  { label: "进入实时会话", href: "/conversation" },
  { label: "查看历史记录", href: "/sessions" },
  { label: "登录团队空间", href: "/login" },
];

function HomeHero() {
  return (
    <SectionBlock>
      <ResponsiveGrid variant="split" className="items-start">
        <div className="flex flex-col gap-5">
          <SectionHeader
            overline="Spinner 控制台"
            title="对话、协作与观测的一体化界面"
            description="参考常见云端产品的控制台信息架构，保持少量文案与清晰入口。"
            titleElement="h1"
            actions={
              <div className="flex w-full flex-wrap gap-2 sm:w-auto">
                <Link href="/conversation" className="w-full sm:w-auto">
                  <Button className="w-full sm:w-auto">开始对话</Button>
                </Link>
                <Link href="/login" className="w-full sm:w-auto">
                  <Button variant="outline" className="w-full sm:w-auto">
                    登录团队
                  </Button>
                </Link>
              </div>
            }
          />
          <ul className="grid gap-3 sm:grid-cols-2">
            <li className="flex flex-col gap-1">
              <p>统一流</p>
              <p>消息、工具响应与事件时间线保持同一个视角。</p>
            </li>
            <li className="flex flex-col gap-1">
              <p>随时重访</p>
              <p>历史记录、登录入口和实时会话统一在首屏。</p>
            </li>
          </ul>
        </div>
        <Card className="w-full">
          <CardHeader className="gap-2">
            <CardTitle>常用入口</CardTitle>
            <CardDescription>按需进入实时对话、历史或团队空间。</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-2">
            {quickLinks.map((item) => (
              <Link key={item.href} href={item.href} className="w-full">
                <Button variant="outline" className="w-full">
                  {item.label}
                </Button>
              </Link>
            ))}
          </CardContent>
        </Card>
      </ResponsiveGrid>
    </SectionBlock>
  );
}

function FeatureSection() {
  return (
    <SectionBlock>
      <SectionHeader
        title="核心体验"
        description="基于行业控制台常见模块，突出实时、协同与可复用。"
      />
      <ResponsiveGrid className="grid-cols-1" variant="three">
        {featureCards.map((item) => (
          <Card key={item.title} className="h-full">
            <CardHeader className="gap-2">
              <CardTitle>{item.title}</CardTitle>
              <CardDescription>{item.description}</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Link href={item.action.href} className="w-full sm:w-auto">
                <Button variant="outline" className="w-full sm:w-auto">
                  {item.action.label}
                </Button>
              </Link>
            </CardContent>
          </Card>
        ))}
      </ResponsiveGrid>
    </SectionBlock>
  );
}

function WorkflowSection() {
  return (
    <SectionBlock>
      <SectionHeader title="三步上手" description="遵循常见云端产品的引导节奏。" />
      <ResponsiveGrid className="grid-cols-1" variant="three">
        {workflow.map((item, index) => (
          <Card key={item.title} className="h-full">
            <CardHeader className="gap-2">
              <CardTitle>
                {index + 1}. {item.title}
              </CardTitle>
              <CardDescription>{item.description}</CardDescription>
            </CardHeader>
          </Card>
        ))}
      </ResponsiveGrid>
    </SectionBlock>
  );
}

function QuickStartSection() {
  return (
    <SectionBlock>
      <Card>
        <CardHeader className="gap-2 text-center">
          <CardTitle>立即使用</CardTitle>
          <CardDescription>常见操作集中在一个卡片中，桌面与移动端一致。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap justify-center gap-3">
          <Link href="/login" className="w-full sm:w-auto">
            <Button className="w-full sm:w-auto">登录团队</Button>
          </Link>
          <Link href="/conversation" className="w-full sm:w-auto">
            <Button variant="outline" className="w-full sm:w-auto">
              打开会话
            </Button>
          </Link>
          <Link href="/sessions" className="w-full sm:w-auto">
            <Button variant="outline" className="w-full sm:w-auto">
              浏览历史
            </Button>
          </Link>
        </CardContent>
      </Card>
    </SectionBlock>
  );
}

export default function HomePage() {
  return (
    <PageShell>
      <PageContainer>
        <HomeHero />
        <FeatureSection />
        <WorkflowSection />
        <QuickStartSection />
      </PageContainer>
    </PageShell>
  );
}
