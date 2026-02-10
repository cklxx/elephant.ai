# Claude Code 3D 建筑工地可视化

**创建时间**: 2026-02-11 00:45
**计划类型**: 功能增强（重构可视化界面）
**预计周期**: 2-3 天
**优先级**: P1

---

## Context

### 为什么重构

当前的 FolderTreemap 可视化虽然功能完整，但：
1. **不够直观**：Treemap 抽象程度高，用户难以理解
2. **缺乏趣味性**：没有"建造"的感觉
3. **视觉吸引力不足**：2D 平面展示比较单调

用户希望：
- **3D 建筑工地风格**（类似 Minecraft）
- **建造过程可视化**（从无到有）
- **丰富的动画效果**（粒子、热力图、工具标记）

### 设计目标

创建一个 **3D 建筑工地可视化界面**，让 Claude Code 的工作过程就像在建造一座代码城市：
- 文件夹 = 建筑物（体素堆叠）
- 文件 = 砖块（Minecraft 方块）
- Claude Code = 建筑工人螃蟹
- 操作 = 施工动作（挥锤、看图纸、电钻等）

---

## 技术选型

### 核心库

| 库 | 用途 | 版本 |
|----|------|------|
| **three** | 3D 渲染引擎 | ^0.169.0 |
| **@react-three/fiber** | React Three.js 集成 | ^8.18.5 |
| **@react-three/drei** | 辅助组件（控制器、文字等） | ^9.120.6 |
| **@react-three/postprocessing** | 后处理效果（可选） | ^2.16.4 |

### 为什么选择 Three.js

1. **成熟生态**：大量现成的工具和示例
2. **性能优秀**：WebGL 渲染，支持数千个对象
3. **React 友好**：R3F 提供声明式 API
4. **易于调试**：浏览器开发工具支持好

### 替代方案对比

| 方案 | 优点 | 缺点 | 选择理由 |
|------|------|------|---------|
| **Three.js** ✅ | 成熟、性能好、生态丰富 | 学习曲线陡峭 | 最佳平衡 |
| Babylon.js | 功能更强大 | 体积大、React 集成较差 | 过于重量级 |
| Unity WebGL | 专业游戏引擎 | 编译慢、体积巨大 | 不适合 Web |
| CSS 3D | 简单易用 | 性能差、效果有限 | 无法满足需求 |

---

## 设计规范

### 建筑物生成规则

#### 1. 布局算法
```
输入：文件夹列表 folders[]
输出：建筑物位置 positions[]

算法：螺旋布局
- 中心点：(0, 0)
- 半径：根据文件夹数量计算
- 间距：max(建筑宽度) * 1.5

伪代码：
for i, folder in enumerate(folders):
  angle = i * golden_angle  // 黄金角度 137.5°
  radius = sqrt(i) * spacing
  x = radius * cos(angle)
  z = radius * sin(angle)
  y = 0
```

#### 2. 建筑高度
```typescript
// 高度由文件数量和代码行数决定
height = Math.min(
  Math.log(fileCount + 1) * 2,  // 对数缩放，避免过高
  10                             // 最大 10 层
)

// 底部面积
width = Math.sqrt(fileCount) * 0.5
depth = width
```

#### 3. 体素化风格
- **方块大小**：1x1x1 单位
- **堆叠方式**：从底部向上，类似搭积木
- **间隙**：方块间有 0.05 单位的缝隙（显示边缘）

### 颜色系统

#### 活跃度热力图
```typescript
// 根据最近操作次数计算热度
function getHeatColor(activityScore: number): THREE.Color {
  // activityScore: 0 (冷) -> 1 (热)

  if (activityScore < 0.2) {
    return new THREE.Color(0x5588aa); // 冷蓝色
  } else if (activityScore < 0.5) {
    return new THREE.Color(0x88aa77); // 温绿色
  } else if (activityScore < 0.8) {
    return new THREE.Color(0xddaa44); // 暖黄色
  } else {
    return new THREE.Color(0xff5533); // 热红色
  }
}

// 活跃度计算
activityScore = recentOperations.length / MAX_OPERATIONS
```

#### 工具标记颜色
- **Read**: 蓝色 `#4A90E2`
- **Write**: 绿色 `#7ED321`
- **Edit**: 黄色 `#F5A623`
- **Grep**: 紫色 `#9013FE`
- **Glob**: 靛蓝 `#4A4A4A`
- **Bash**: 橙色 `#F8E71C`

### 动画参数

| 效果 | 持续时间 | 缓动函数 | 备注 |
|------|---------|---------|------|
| 方块出现 | 0.3s | easeOutBounce | 弹跳效果 |
| 建筑物高亮 | 0.5s | easeInOutQuad | 淡入淡出 |
| 螃蟹移动 | 1.0s | easeInOutCubic | 平滑移动 |
| 粒子寿命 | 2.0s | linear | 逐渐消失 |
| 工具标记 | 10s | easeOut | 淡出消失 |

---

## 实施计划

### Phase 1: 基础 3D 场景（Day 1 上午）

#### 1.1 安装依赖
```bash
cd web
npm install three @react-three/fiber @react-three/drei
```

#### 1.2 创建 3D 场景组件
**文件**: `web/components/visualizer/ConstructionSite3D.tsx`

```typescript
'use client';

import { Canvas } from '@react-three/fiber';
import { OrbitControls, Grid, Sky } from '@react-three/drei';

export function ConstructionSite3D() {
  return (
    <Canvas
      camera={{ position: [20, 15, 20], fov: 60 }}
      shadows
      dpr={[1, 2]} // 设备像素比
    >
      {/* 环境光 */}
      <ambientLight intensity={0.3} />
      <directionalLight
        position={[10, 20, 5]}
        intensity={0.8}
        castShadow
        shadow-mapSize={[2048, 2048]}
      />

      {/* 天空 */}
      <Sky sunPosition={[100, 20, 100]} />

      {/* 地面网格 */}
      <Grid infiniteGrid cellSize={1} sectionSize={5} fadeDistance={50} />

      {/* 相机控制 */}
      <OrbitControls
        enablePan={true}
        enableZoom={true}
        enableRotate={true}
        minPolarAngle={0}
        maxPolarAngle={Math.PI / 2.1}
      />

      {/* 建筑物容器 */}
      <Buildings />
    </Canvas>
  );
}
```

#### 1.3 创建体素建筑组件
**文件**: `web/components/visualizer/Building.tsx`

```typescript
import { useRef } from 'react';
import { Mesh } from 'three';
import { Box } from '@react-three/drei';

interface BuildingProps {
  position: [number, number, number];
  height: number;
  width: number;
  color: string;
  opacity?: number;
}

export function Building({ position, height, width, color, opacity = 1 }: BuildingProps) {
  const meshRef = useRef<Mesh>(null);

  // 堆叠方块
  const blocks = Array.from({ length: Math.floor(height) }, (_, i) => i);

  return (
    <group position={position}>
      {blocks.map((level) => (
        <Box
          key={level}
          position={[0, level + 0.5, 0]}
          args={[width, 1, width]}
          castShadow
          receiveShadow
        >
          <meshStandardMaterial
            color={color}
            opacity={opacity}
            transparent={opacity < 1}
            roughness={0.7}
            metalness={0.2}
          />
        </Box>
      ))}
    </group>
  );
}
```

### Phase 2: 建筑物布局和数据绑定（Day 1 下午）

#### 2.1 螺旋布局算法
**文件**: `web/lib/visualizer/layout.ts`

```typescript
export interface BuildingLayout {
  id: string;
  position: [number, number, number];
  height: number;
  width: number;
  folderPath: string;
}

const GOLDEN_ANGLE = 137.5 * (Math.PI / 180);
const SPACING = 3;

export function generateSpiralLayout(
  folders: FolderInfo[]
): BuildingLayout[] {
  return folders.map((folder, index) => {
    const angle = index * GOLDEN_ANGLE;
    const radius = Math.sqrt(index) * SPACING;
    const x = radius * Math.cos(angle);
    const z = radius * Math.sin(angle);

    const height = Math.min(Math.log(folder.fileCount + 1) * 2, 10);
    const width = Math.sqrt(folder.fileCount) * 0.3 + 0.5;

    return {
      id: folder.path,
      position: [x, 0, z],
      height,
      width,
      folderPath: folder.path,
    };
  });
}
```

#### 2.2 集成文件夹数据
**修改**: `web/components/visualizer/ConstructionSite3D.tsx`

```typescript
import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { generateSpiralLayout } from '@/lib/visualizer/layout';

function Buildings() {
  // 获取文件夹数据
  const { data: foldersData } = useQuery({
    queryKey: ['visualizer-folders'],
    queryFn: async () => {
      const res = await fetch('/api/visualizer/folders?depth=4');
      return res.json();
    },
    refetchInterval: 30000, // 30 秒刷新
  });

  // 生成布局
  const buildings = useMemo(() => {
    if (!foldersData?.folders) return [];
    return generateSpiralLayout(foldersData.folders);
  }, [foldersData]);

  return (
    <>
      {buildings.map((building) => (
        <Building
          key={building.id}
          position={building.position}
          height={building.height}
          width={building.width}
          color="#88aa77"
        />
      ))}
    </>
  );
}
```

### Phase 3: 建造进度动画（Day 1 晚上）

#### 3.1 添加建造动画
**修改**: `web/components/visualizer/Building.tsx`

```typescript
import { useSpring, animated } from '@react-spring/three';

export function Building({ ... }: BuildingProps) {
  const [isBuilt, setIsBuilt] = useState(false);

  // 建造动画
  const { scale } = useSpring({
    scale: isBuilt ? 1 : 0,
    config: { tension: 120, friction: 14 },
  });

  useEffect(() => {
    // 触发建造（首次访问时）
    if (shouldBuild) {
      setIsBuilt(true);
    }
  }, [shouldBuild]);

  return (
    <group position={position}>
      {blocks.map((level, i) => (
        <animated.mesh
          key={level}
          position={[0, level + 0.5, 0]}
          scale-y={scale.to((s) => Math.max(0, s - i * 0.1))} // 从下往上
        >
          <boxGeometry args={[width, 1, width]} />
          <meshStandardMaterial color={color} />
        </animated.mesh>
      ))}
    </group>
  );
}
```

### Phase 4: 建筑工人螃蟹（Day 2 上午）

#### 4.1 3D 螃蟹模型
**文件**: `web/components/visualizer/CrabWorker.tsx`

```typescript
import { useFrame } from '@react-three/fiber';
import { useSpring, animated } from '@react-spring/three';

export function CrabWorker({ targetPosition, currentTool }) {
  const meshRef = useRef();

  // 移动动画
  const { position } = useSpring({
    position: targetPosition,
    config: { tension: 80, friction: 20 },
  });

  // 挥动钳子动画
  useFrame(({ clock }) => {
    if (currentTool === 'Write' && meshRef.current) {
      meshRef.current.children[0].rotation.z = Math.sin(clock.elapsedTime * 5) * 0.5;
    }
  });

  return (
    <animated.group position={position} ref={meshRef}>
      {/* 身体 */}
      <mesh position={[0, 0.5, 0]}>
        <boxGeometry args={[1, 0.8, 1.2]} />
        <meshStandardMaterial color="#e67e22" />
      </mesh>

      {/* 安全帽 */}
      <mesh position={[0, 1.1, 0]}>
        <cylinderGeometry args={[0.5, 0.6, 0.3, 8]} />
        <meshStandardMaterial color="#f39c12" />
      </mesh>

      {/* 左钳子 */}
      <mesh position={[-0.7, 0.5, 0]}>
        <boxGeometry args={[0.4, 0.3, 0.2]} />
        <meshStandardMaterial color="#d35400" />
      </mesh>

      {/* 右钳子 */}
      <mesh position={[0.7, 0.5, 0]}>
        <boxGeometry args={[0.4, 0.3, 0.2]} />
        <meshStandardMaterial color="#d35400" />
      </mesh>

      {/* 眼睛 */}
      <mesh position={[-0.2, 0.7, 0.6]}>
        <sphereGeometry args={[0.1, 8, 8]} />
        <meshStandardMaterial color="#fff" />
      </mesh>
      <mesh position={[0.2, 0.7, 0.6]}>
        <sphereGeometry args={[0.1, 8, 8]} />
        <meshStandardMaterial color="#fff" />
      </mesh>
    </animated.group>
  );
}
```

### Phase 5: 粒子系统（Day 2 下午）

#### 5.1 施工粒子效果
**文件**: `web/components/visualizer/ParticleEffects.tsx`

```typescript
import { useMemo, useRef } from 'react';
import { useFrame } from '@react-three/fiber';
import * as THREE from 'three';

export function DustParticles({ origin, active }) {
  const particlesRef = useRef();
  const count = 50;

  const [positions, velocities] = useMemo(() => {
    const positions = new Float32Array(count * 3);
    const velocities = new Float32Array(count * 3);

    for (let i = 0; i < count; i++) {
      positions[i * 3] = origin[0] + (Math.random() - 0.5) * 2;
      positions[i * 3 + 1] = origin[1];
      positions[i * 3 + 2] = origin[2] + (Math.random() - 0.5) * 2;

      velocities[i * 3] = (Math.random() - 0.5) * 0.05;
      velocities[i * 3 + 1] = Math.random() * 0.1;
      velocities[i * 3 + 2] = (Math.random() - 0.5) * 0.05;
    }

    return [positions, velocities];
  }, [origin]);

  useFrame((state, delta) => {
    if (!active || !particlesRef.current) return;

    const positions = particlesRef.current.geometry.attributes.position.array;

    for (let i = 0; i < count; i++) {
      positions[i * 3] += velocities[i * 3];
      positions[i * 3 + 1] += velocities[i * 3 + 1];
      positions[i * 3 + 2] += velocities[i * 3 + 2];

      velocities[i * 3 + 1] -= 0.002; // 重力
    }

    particlesRef.current.geometry.attributes.position.needsUpdate = true;
  });

  return (
    <points ref={particlesRef}>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={count}
          array={positions}
          itemSize={3}
        />
      </bufferGeometry>
      <pointsMaterial size={0.1} color="#aaaaaa" opacity={0.6} transparent />
    </points>
  );
}
```

### Phase 6: 活跃度热力图（Day 2 晚上）

#### 6.1 热度计算
**文件**: `web/lib/visualizer/heatmap.ts`

```typescript
export class HeatmapManager {
  private activityMap = new Map<string, number[]>(); // path -> timestamps
  private readonly WINDOW_SIZE = 60000; // 1 分钟窗口

  recordActivity(path: string) {
    const now = Date.now();
    const activities = this.activityMap.get(path) || [];

    // 添加新活动
    activities.push(now);

    // 移除过期活动
    const filtered = activities.filter((t) => now - t < this.WINDOW_SIZE);
    this.activityMap.set(path, filtered);
  }

  getHeatScore(path: string): number {
    const activities = this.activityMap.get(path) || [];
    return Math.min(activities.length / 10, 1); // 归一化到 0-1
  }

  getHeatColor(score: number): string {
    if (score < 0.2) return '#5588aa';
    if (score < 0.5) return '#88aa77';
    if (score < 0.8) return '#ddaa44';
    return '#ff5533';
  }
}
```

### Phase 7: 工具标记系统（Day 3 上午）

#### 7.1 浮动工具图标
**文件**: `web/components/visualizer/ToolMarker.tsx`

```typescript
import { Html } from '@react-three/drei';
import { animated, useSpring } from '@react-spring/three';

export function ToolMarker({ position, tool, duration = 10000 }) {
  const [visible, setVisible] = useState(true);

  const { opacity } = useSpring({
    from: { opacity: 1 },
    to: { opacity: 0 },
    config: { duration },
    onRest: () => setVisible(false),
  });

  if (!visible) return null;

  const iconMap = {
    Read: '📋',
    Write: '🔨',
    Edit: '⚡',
    Grep: '🔦',
    Glob: '🚁',
    Bash: '🚧',
  };

  return (
    <Html position={[position[0], position[1] + 2, position[2]]} center>
      <animated.div
        style={{
          opacity,
          fontSize: '2rem',
          filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.3))',
        }}
      >
        {iconMap[tool] || '⚙️'}
      </animated.div>
    </Html>
  );
}
```

### Phase 8: 集成和优化（Day 3 下午）

#### 8.1 更新主可视化页面
**修改**: `web/app/visualizer/page.tsx`

```typescript
import { ConstructionSite3D } from '@/components/visualizer/ConstructionSite3D';

export default function VisualizerPage() {
  return (
    <div className="fixed inset-0 bg-gradient-to-b from-sky-200 to-sky-400">
      {/* Header */}
      <header className="absolute top-0 left-0 right-0 z-10 bg-white/80 backdrop-blur">
        <div className="px-6 py-3 flex items-center justify-between">
          <h1 className="text-xl font-bold">🏗️ Claude Code Construction Site</h1>
          <ConnectionStatus />
        </div>
      </header>

      {/* 3D Scene */}
      <ConstructionSite3D />

      {/* Event Log Sidebar */}
      <div className="absolute right-4 top-20 bottom-4 w-80">
        <EventLog events={events} />
      </div>
    </div>
  );
}
```

#### 8.2 性能优化
- **LOD (Level of Detail)**: 远距离建筑使用低多边形模型
- **实例化渲染**: 相同方块使用 InstancedMesh
- **视锥剔除**: 只渲染可见建筑
- **粒子池**: 复用粒子对象，避免频繁创建

---

## 验证清单

### 功能验证
- [ ] 建筑物正确布局（螺旋排列）
- [ ] 建造动画流畅（方块逐个出现）
- [ ] 螃蟹移动到正确位置
- [ ] 粒子效果正常显示
- [ ] 热力图颜色正确更新
- [ ] 工具标记正常显示和淡出
- [ ] 相机控制流畅（旋转、缩放、平移）

### 性能验证
- [ ] 60 FPS（100 个建筑）
- [ ] 30 FPS（500 个建筑）
- [ ] 内存占用 < 500MB
- [ ] 初始加载 < 3 秒

### 兼容性验证
- [ ] Chrome/Edge（最新版）
- [ ] Firefox（最新版）
- [ ] Safari 14+（可能需要降级部分效果）

---

## 风险和缓解

### 风险 1: 性能问题（大型项目）
**缓解**:
- 限制最大建筑数量（Top 200 活跃文件夹）
- 使用实例化渲染
- 添加 LOD 系统

### 风险 2: 3D 学习曲线陡峭
**缓解**:
- 复用 Three.js 官方示例
- 使用 @react-three/drei 简化常见任务
- 分阶段实施，先实现基础再添加特效

### 风险 3: Safari WebGL 兼容性
**缓解**:
- 测试时优先检查 Safari
- 降级复杂效果（粒子、后处理）
- 提供 2D fallback 选项

---

## 成功标准

1. **视觉冲击力**: 用户看到界面会说"哇！"
2. **直观易懂**: 5 秒内理解建筑 = 代码模块
3. **流畅交互**: 60 FPS 无卡顿
4. **信息丰富**: 可以看到热力图、工具标记、建造进度

---

## 参考资源

- [Three.js 官方示例](https://threejs.org/examples/)
- [React Three Fiber 文档](https://docs.pmnd.rs/react-three-fiber)
- [Minecraft 体素风格参考](https://www.youtube.com/watch?v=dQw4w9WgXcQ)
- [建筑可视化案例](https://github.com/mrdoob/three.js/blob/master/examples/webgl_geometry_minecraft.html)
