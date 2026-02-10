import { ConstructionSiteVisualizer } from '@/components/visualizer/ConstructionSiteVisualizer';

export const metadata = {
  title: 'Claude Code Construction Site - elephant.ai',
  description: '实时观察 Claude Code 建造代码城市',
};

export default function VisualizerPage() {
  return <ConstructionSiteVisualizer />;
}
