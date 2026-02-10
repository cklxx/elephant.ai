import { CodeVisualizer } from '@/components/visualizer/CodeVisualizer';

export const metadata = {
  title: 'Claude Code Visualizer - elephant.ai',
  description: '实时观察 Claude Code 在代码库中的工作过程',
};

export default function VisualizerPage() {
  return <CodeVisualizer />;
}
