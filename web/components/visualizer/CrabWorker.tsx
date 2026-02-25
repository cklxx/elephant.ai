'use client';

import { useRef, useMemo } from 'react';
import { Group, Mesh } from 'three';
import { useFrame } from '@react-three/fiber';
import { useSpring, animated, config } from '@react-spring/three';
import { Html } from '@react-three/drei';
import { VisualizerEvent } from '@/hooks/useVisualizerStream';

interface CrabWorkerProps {
  currentEvent: VisualizerEvent | null;
  targetPosition: [number, number, number] | null;
}

export function CrabWorker({ currentEvent, targetPosition }: CrabWorkerProps) {
  const groupRef = useRef<Group>(null);
  const leftClawRef = useRef<Mesh>(null);
  const rightClawRef = useRef<Mesh>(null);

  // Default position (hovering in the center when idle)
  const defaultPosition: [number, number, number] = [0, 5, 0];
  const position = targetPosition || defaultPosition;

  // Smooth movement animation
  const { pos } = useSpring({
    pos: position,
    config: config.slow,
  });

  // Tool-specific animations
  const isWorking = currentEvent?.status === 'started';
  const tool = currentEvent?.tool || 'Idle';

  // Animate claws based on tool
  useFrame(({ clock }) => {
    if (!leftClawRef.current || !rightClawRef.current) return;

    switch (tool) {
      case 'Write':
        // Hammering motion
        const hammerAngle = isWorking ? Math.sin(clock.elapsedTime * 10) * 0.5 : 0;
        leftClawRef.current.rotation.z = hammerAngle;
        rightClawRef.current.rotation.z = -hammerAngle;
        break;

      case 'Edit':
        // Drilling motion (fast rotation)
        if (isWorking) {
          leftClawRef.current.rotation.y += 0.5;
          rightClawRef.current.rotation.y -= 0.5;
        }
        break;

      case 'Read':
        // Waving motion (gentle)
        const waveAngle = Math.sin(clock.elapsedTime * 2) * 0.2;
        leftClawRef.current.rotation.z = waveAngle;
        rightClawRef.current.rotation.z = -waveAngle;
        break;

      default:
        // Idle breathing motion
        const breatheAngle = Math.sin(clock.elapsedTime * 1.5) * 0.1;
        leftClawRef.current.rotation.z = breatheAngle;
        rightClawRef.current.rotation.z = -breatheAngle;
    }

    // Bobbing motion when working
    if (groupRef.current && isWorking) {
      groupRef.current.position.y = position[1] + Math.sin(clock.elapsedTime * 3) * 0.2;
    }
  });

  // Tool icon for speech bubble
  const toolIcon = useMemo(() => {
    const iconMap: Record<string, string> = {
      Read: '📋',
      Write: '🔨',
      Edit: '⚡',
      Grep: '🔦',
      Glob: '🚁',
      Bash: '🚧',
      Idle: '😴',
    };
    return iconMap[tool] || '⚙️';
  }, [tool]);

  const actionText = useMemo(() => {
    const textMap: Record<string, string> = {
      Read: '查看图纸',
      Write: '砌砖搭建',
      Edit: '修补装修',
      Grep: '扫描检查',
      Glob: '巡查工地',
      Bash: '操作机械',
      Idle: '休息中',
    };
    return textMap[tool] || '施工中';
  }, [tool]);

  return (
    <animated.group ref={groupRef} position={pos as any}>
      {/* Body */}
      <mesh position={[0, 0, 0]} castShadow>
        <boxGeometry args={[1, 0.8, 1.2]} />
        <meshStandardMaterial color="#e67e22" roughness={0.6} metalness={0.3} />
      </mesh>

      {/* Safety helmet */}
      <mesh position={[0, 0.6, 0]} castShadow>
        <cylinderGeometry args={[0.5, 0.6, 0.3, 8]} />
        <meshStandardMaterial color="#f39c12" roughness={0.4} metalness={0.6} />
      </mesh>

      {/* Helmet light */}
      <mesh position={[0, 0.65, 0.5]}>
        <sphereGeometry args={[0.08, 8, 8]} />
        <meshStandardMaterial
          color={isWorking ? '#ffff00' : '#666666'}
          emissive={isWorking ? '#ffff00' : '#000000'}
          emissiveIntensity={isWorking ? 1 : 0}
        />
      </mesh>

      {/* Eyes */}
      <mesh position={[-0.2, 0.15, 0.6]}>
        <sphereGeometry args={[0.12, 8, 8]} />
        <meshStandardMaterial color="#fff" />
      </mesh>
      <mesh position={[0.2, 0.15, 0.6]}>
        <sphereGeometry args={[0.12, 8, 8]} />
        <meshStandardMaterial color="#fff" />
      </mesh>

      {/* Pupils */}
      <mesh position={[-0.2, 0.15, 0.7]}>
        <sphereGeometry args={[0.06, 8, 8]} />
        <meshStandardMaterial color="#000" />
      </mesh>
      <mesh position={[0.2, 0.15, 0.7]}>
        <sphereGeometry args={[0.06, 8, 8]} />
        <meshStandardMaterial color="#000" />
      </mesh>

      {/* Left claw */}
      <mesh ref={leftClawRef} position={[-0.7, 0, 0.3]} castShadow>
        <boxGeometry args={[0.4, 0.3, 0.25]} />
        <meshStandardMaterial color="#d35400" roughness={0.7} />
      </mesh>

      {/* Left claw pincers */}
      <mesh position={[-0.9, 0, 0.3]}>
        <boxGeometry args={[0.15, 0.15, 0.1]} />
        <meshStandardMaterial color="#c0392b" />
      </mesh>

      {/* Right claw */}
      <mesh ref={rightClawRef} position={[0.7, 0, 0.3]} castShadow>
        <boxGeometry args={[0.4, 0.3, 0.25]} />
        <meshStandardMaterial color="#d35400" roughness={0.7} />
      </mesh>

      {/* Right claw pincers */}
      <mesh position={[0.9, 0, 0.3]}>
        <boxGeometry args={[0.15, 0.15, 0.1]} />
        <meshStandardMaterial color="#c0392b" />
      </mesh>

      {/* Legs */}
      {[-0.3, -0.1, 0.1, 0.3].map((x, i) => (
        <mesh key={i} position={[x, -0.5, 0]} castShadow>
          <cylinderGeometry args={[0.05, 0.05, 0.4, 6]} />
          <meshStandardMaterial color="#d35400" />
        </mesh>
      ))}

      {/* Speech bubble with tool icon */}
      {currentEvent && (
        <Html position={[0, 1.5, 0]} center distanceFactor={10}>
          <div className="bg-white rounded-lg shadow-xl px-3 py-2 min-w-[120px] border-2 border-gray-300 animate-fadeInOut">
            <div className="flex items-center gap-2 justify-center">
              <span className="text-2xl">{toolIcon}</span>
              <div>
                <div className="text-xs font-semibold text-gray-900">{actionText}</div>
                {currentEvent.path && (
                  <div className="text-[10px] text-gray-600 truncate max-w-[100px]">
                    {currentEvent.path.split('/').pop()}
                  </div>
                )}
              </div>
            </div>

            {/* Bubble tail */}
            <div className="absolute left-1/2 bottom-[-8px] -translate-x-1/2 w-0 h-0 border-l-[8px] border-l-transparent border-r-[8px] border-r-transparent border-t-[8px] border-t-white" />
          </div>
        </Html>
      )}

      {/* Working light effect */}
      {isWorking && (
        <pointLight intensity={1} distance={5} color="#f39c12" />
      )}
    </animated.group>
  );
}
