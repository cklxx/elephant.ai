import type {CSSProperties, ReactNode} from 'react';
import {
  AbsoluteFill,
  Easing,
  Img,
  Sequence,
  interpolate,
  spring,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from 'remotion';

const colors = {
  bg: '#061722',
  deep: '#0a2b40',
  accent: '#16a34a',
  accentSoft: '#4ade80',
  textMain: '#ecfeff',
  textSub: '#bae6fd',
  panel: 'rgba(9, 37, 55, 0.75)',
  border: 'rgba(125, 211, 252, 0.22)',
};

const full: CSSProperties = {
  width: '100%',
  height: '100%',
};

const cardBase: CSSProperties = {
  background: colors.panel,
  border: `1px solid ${colors.border}`,
  borderRadius: 24,
  boxShadow: '0 30px 60px rgba(2, 12, 26, 0.45)',
  backdropFilter: 'blur(4px)',
};

const titleStyle: CSSProperties = {
  fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
  color: colors.textMain,
  fontSize: 86,
  fontWeight: 700,
  lineHeight: 1.06,
  letterSpacing: -2,
};

const subtitleStyle: CSSProperties = {
  fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
  color: colors.textSub,
  fontSize: 36,
  fontWeight: 500,
  lineHeight: 1.4,
};

const FloatingGlow = () => {
  const frame = useCurrentFrame();
  const drift = Math.sin(frame / 28) * 24;

  return (
    <>
      <div
        style={{
          position: 'absolute',
          top: 120 + drift,
          left: 180,
          width: 460,
          height: 460,
          borderRadius: 999,
          background: 'radial-gradient(circle, rgba(22,163,74,0.45), rgba(22,163,74,0.02) 70%)',
          filter: 'blur(8px)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          right: 140,
          bottom: 80 - drift,
          width: 520,
          height: 520,
          borderRadius: 999,
          background: 'radial-gradient(circle, rgba(56,189,248,0.38), rgba(56,189,248,0.02) 70%)',
          filter: 'blur(12px)',
        }}
      />
    </>
  );
};

const SceneFrame = ({children}: {children: ReactNode}) => {
  return (
    <AbsoluteFill
      style={{
        ...full,
        background:
          'linear-gradient(135deg, #05121d 0%, #052033 42%, #09324a 100%)',
      }}
    >
      <FloatingGlow />
      <div
        style={{
          ...full,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          padding: '78px 92px',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {children}
      </div>
    </AbsoluteFill>
  );
};

const IntroScene = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();

  const lift = interpolate(frame, [0, 70], [52, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.cubic),
  });

  const fadeIn = interpolate(frame, [0, 22], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  const logoPop = spring({
    frame,
    fps,
    config: {
      damping: 12,
      stiffness: 110,
      mass: 0.8,
    },
  });

  return (
    <SceneFrame>
      <div
        style={{
          ...cardBase,
          height: 860,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '80px 84px',
          opacity: fadeIn,
          transform: `translateY(${lift}px)`,
        }}
      >
        <div style={{maxWidth: 1100}}>
          <div style={{...subtitleStyle, fontSize: 28, letterSpacing: 4, textTransform: 'uppercase'}}>
            Project Promo Demo
          </div>
          <div style={{...titleStyle, marginTop: 22}}>elephant.ai</div>
          <div style={{...subtitleStyle, marginTop: 28, maxWidth: 980}}>
            Your AI teammate, always on. Lives in Lark, remembers context, and executes real work autonomously.
          </div>
          <div
            style={{
              marginTop: 42,
              display: 'inline-flex',
              alignItems: 'center',
              gap: 16,
              borderRadius: 999,
              border: `1px solid ${colors.accentSoft}`,
              padding: '12px 24px',
              color: '#dcfce7',
              fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
              fontSize: 28,
              fontWeight: 600,
            }}
          >
            Proactive AI Assistant for Teams
          </div>
        </div>

        <div
          style={{
            width: 360,
            height: 360,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            transform: `scale(${0.8 + logoPop * 0.2}) rotate(${(1 - logoPop) * 8}deg)`,
          }}
        >
          <Img
            src={staticFile('elephant-rounded.png')}
            style={{
              width: 300,
              height: 300,
              borderRadius: 72,
              boxShadow: '0 40px 80px rgba(4, 20, 38, 0.5)',
            }}
          />
        </div>
      </div>
    </SceneFrame>
  );
};

const capabilities = [
  {
    title: 'Persistent Memory',
    body: 'Remembers decisions and conversation context across weeks, so teams never repeat themselves.',
  },
  {
    title: 'Autonomous ReAct Loop',
    body: 'Think → Act → Observe execution handles multi-step tasks without babysitting.',
  },
  {
    title: '15+ Built-in Skills',
    body: 'Research, meeting notes, email, slide decks, and more triggered by natural language.',
  },
  {
    title: 'Approval Gates',
    body: 'Risky operations require explicit human approval before external action.',
  },
];

const CapabilitiesScene = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();

  return (
    <SceneFrame>
      <div
        style={{
          ...subtitleStyle,
          fontSize: 24,
          letterSpacing: 3,
          textTransform: 'uppercase',
          color: '#bbf7d0',
        }}
      >
        Core strengths
      </div>
      <div style={{...titleStyle, marginTop: 12, fontSize: 74}}>Built to move real work</div>
      <div
        style={{
          marginTop: 44,
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gridTemplateRows: '1fr 1fr',
          gap: 24,
          flex: 1,
        }}
      >
        {capabilities.map((cap, i) => {
          const delay = i * 10;
          const progress = spring({
            frame: frame - delay,
            fps,
            config: {
              damping: 14,
              stiffness: 115,
              mass: 0.8,
            },
          });
          const y = interpolate(progress, [0, 1], [30, 0]);
          const opacity = interpolate(progress, [0, 0.35, 1], [0, 0.45, 1]);

          return (
            <div
              key={cap.title}
              style={{
                ...cardBase,
                padding: '34px 34px 30px',
                opacity,
                transform: `translateY(${y}px) scale(${0.94 + 0.06 * progress})`,
              }}
            >
              <div
                style={{
                  fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
                  fontSize: 34,
                  fontWeight: 700,
                  lineHeight: 1.2,
                  color: '#ecfccb',
                  letterSpacing: -0.5,
                }}
              >
                {cap.title}
              </div>
              <div
                style={{
                  marginTop: 18,
                  fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
                  fontSize: 27,
                  fontWeight: 500,
                  lineHeight: 1.42,
                  color: colors.textSub,
                }}
              >
                {cap.body}
              </div>
            </div>
          );
        })}
      </div>
    </SceneFrame>
  );
};

const layers = ['Delivery', 'Application', 'Domain', 'Infrastructure'];

const ArchitectureScene = () => {
  const frame = useCurrentFrame();
  const reveal = interpolate(frame, [0, 130], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.quad),
  });

  return (
    <SceneFrame>
      <div style={{...titleStyle, fontSize: 72}}>Layered architecture, production ready</div>
      <div style={{...subtitleStyle, marginTop: 20, fontSize: 30, maxWidth: 1160}}>
        Clean boundaries make delivery channels, agent logic, and tool adapters evolve independently.
      </div>

      <div
        style={{
          marginTop: 42,
          display: 'flex',
          flexDirection: 'column',
          gap: 18,
          width: 1540,
        }}
      >
        {layers.map((layer, i) => {
          const start = i * 15;
          const local = interpolate(frame, [start, start + 30], [0, 1], {
            extrapolateLeft: 'clamp',
            extrapolateRight: 'clamp',
          });
          return (
            <div
              key={layer}
              style={{
                ...cardBase,
                height: 125,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '0 38px',
                opacity: local,
                transform: `translateX(${interpolate(local, [0, 1], [80, 0])}px)`,
              }}
            >
              <div
                style={{
                  fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
                  fontSize: 44,
                  fontWeight: 700,
                  color: '#f0fdf4',
                  letterSpacing: -0.8,
                }}
              >
                {layer}
              </div>
              <div
                style={{
                  width: `${Math.max(16, 360 * reveal)}px`,
                  height: 10,
                  borderRadius: 999,
                  background: 'linear-gradient(90deg, #34d399, #38bdf8)',
                  boxShadow: '0 0 18px rgba(52, 211, 153, 0.42)',
                }}
              />
            </div>
          );
        })}
      </div>
    </SceneFrame>
  );
};

const CtaScene = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const beat = spring({
    frame,
    fps,
    durationInFrames: 60,
    delay: 8,
    config: {
      damping: 8,
      stiffness: 90,
      mass: 0.6,
    },
  });

  const fade = interpolate(frame, [0, 24], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  return (
    <SceneFrame>
      <div
        style={{
          ...cardBase,
          height: 860,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          textAlign: 'center',
          opacity: fade,
          transform: `scale(${0.94 + beat * 0.06})`,
        }}
      >
        <div style={{...titleStyle, fontSize: 94}}>Ship faster with elephant.ai</div>
        <div style={{...subtitleStyle, marginTop: 26, fontSize: 36, maxWidth: 1180}}>
          Run proactive workflows in Lark, keep long-term memory, and execute with full observability.
        </div>
        <div
          style={{
            marginTop: 40,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 14,
            borderRadius: 999,
            background: 'linear-gradient(90deg, #22c55e, #14b8a6)',
            color: '#052e16',
            fontFamily: '"Plus Jakarta Sans", "Avenir Next", "Helvetica Neue", sans-serif',
            fontSize: 32,
            fontWeight: 700,
            padding: '16px 34px',
            boxShadow: '0 18px 40px rgba(20, 184, 166, 0.35)',
          }}
        >
          github.com/cklxx/elephant.ai
        </div>
      </div>
    </SceneFrame>
  );
};

export const ElephantPromo = () => {
  return (
    <AbsoluteFill style={{backgroundColor: colors.bg}}>
      <Sequence from={0} durationInFrames={150}>
        <IntroScene />
      </Sequence>
      <Sequence from={150} durationInFrames={180}>
        <CapabilitiesScene />
      </Sequence>
      <Sequence from={330} durationInFrames={160}>
        <ArchitectureScene />
      </Sequence>
      <Sequence from={490} durationInFrames={110}>
        <CtaScene />
      </Sequence>
    </AbsoluteFill>
  );
};
