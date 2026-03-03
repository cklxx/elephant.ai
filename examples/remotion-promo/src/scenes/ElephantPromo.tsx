import type {CSSProperties, ReactNode} from "react";
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
} from "remotion";

const palette = {
  bg: "#fafbfc",
  panel: "rgba(255, 255, 255, 0.88)",
  border: "#e2e8f0",
  text: "#0f172a",
  sub: "#64748b",
  indigo: "#6366f1",
  violet: "#8b5cf6",
};

const sectionTitle: CSSProperties = {
  fontFamily: "\"Inter\", \"Plus Jakarta Sans\", sans-serif",
  fontSize: 72,
  lineHeight: 1.08,
  fontWeight: 800,
  color: palette.text,
  letterSpacing: -1.5,
};

const sectionSub: CSSProperties = {
  fontFamily: "\"Inter\", \"Plus Jakarta Sans\", sans-serif",
  fontSize: 32,
  lineHeight: 1.45,
  fontWeight: 500,
  color: palette.sub,
};

const card: CSSProperties = {
  background: palette.panel,
  border: `1px solid ${palette.border}`,
  borderRadius: 28,
  boxShadow: "0 24px 44px rgba(15, 23, 42, 0.08)",
};

const SceneBase = ({children}: {children: ReactNode}) => {
  return (
    <AbsoluteFill
      style={{
        background: palette.bg,
        overflow: "hidden",
      }}
    >
      <div
        style={{
          position: "absolute",
          width: 560,
          height: 560,
          borderRadius: 9999,
          left: -180,
          top: -160,
          background: "radial-gradient(circle, rgba(99,102,241,0.18), rgba(99,102,241,0) 70%)",
          filter: "blur(4px)",
        }}
      />
      <div
        style={{
          position: "absolute",
          width: 620,
          height: 620,
          borderRadius: 9999,
          right: -200,
          bottom: -180,
          background: "radial-gradient(circle, rgba(139,92,246,0.16), rgba(139,92,246,0) 70%)",
          filter: "blur(4px)",
        }}
      />
      <div
        style={{
          position: "absolute",
          inset: 0,
          padding: "72px 84px",
          display: "flex",
          flexDirection: "column",
        }}
      >
        {children}
      </div>
    </AbsoluteFill>
  );
};

const Intro = () => {
  const frame = useCurrentFrame();
  const y = interpolate(frame, [0, 36], [32, 0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.out(Easing.cubic),
  });
  const opacity = interpolate(frame, [0, 24], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  return (
    <SceneBase>
      <div
        style={{
          ...card,
          padding: 48,
          display: "flex",
          flexDirection: "column",
          gap: 28,
          opacity,
          transform: `translateY(${y}px)`,
        }}
      >
        <Img
          src={staticFile("home-banner.png")}
          style={{
            width: "100%",
            height: 370,
            borderRadius: 20,
            objectFit: "cover",
            border: `1px solid ${palette.border}`,
          }}
        />
        <div style={{fontFamily: "\"Inter\", sans-serif", fontSize: 24, color: palette.indigo, fontWeight: 700}}>
          elephant.ai project promo
        </div>
        <div style={sectionTitle}>Your AI teammate, always on.</div>
        <div style={{...sectionSub, fontSize: 30}}>
          Proactive assistant in Lark with persistent memory, autonomous execution, and production-grade safety gates.
        </div>
      </div>
    </SceneBase>
  );
};

const highlights = [
  {
    title: "Persistent memory",
    desc: "Keeps decisions and context across sessions so teams stop repeating the same setup.",
  },
  {
    title: "Autonomous ReAct loop",
    desc: "Think, act, observe workflows complete multi-step tasks with minimal supervision.",
  },
  {
    title: "Approval gates",
    desc: "External or risky operations require explicit human sign-off before execution.",
  },
];

const Highlights = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();

  return (
    <SceneBase>
      <div style={{fontFamily: "\"Inter\", sans-serif", fontSize: 24, color: palette.indigo, fontWeight: 700}}>
        Capabilities
      </div>
      <div style={{...sectionTitle, marginTop: 14}}>Everything an agent should be.</div>
      <div style={{...sectionSub, marginTop: 18}}>
        Same style as homepage. Same product story. One consistent voice.
      </div>
      <div style={{marginTop: 40, display: "grid", gap: 20}}>
        {highlights.map((item, i) => {
          const s = spring({
            frame: frame - i * 10,
            fps,
            config: {damping: 14, stiffness: 110},
          });
          const localOpacity = interpolate(s, [0, 1], [0, 1]);
          const localY = interpolate(s, [0, 1], [24, 0]);
          return (
            <div
              key={item.title}
              style={{
                ...card,
                padding: "24px 26px",
                opacity: localOpacity,
                transform: `translateY(${localY}px)`,
              }}
            >
              <div
                style={{
                  fontFamily: "\"Inter\", sans-serif",
                  fontSize: 38,
                  lineHeight: 1.25,
                  fontWeight: 700,
                  color: palette.text,
                }}
              >
                {item.title}
              </div>
              <div style={{...sectionSub, marginTop: 8, fontSize: 27}}>{item.desc}</div>
            </div>
          );
        })}
      </div>
    </SceneBase>
  );
};

const layers = ["Delivery", "Application", "Domain", "Infrastructure"];

const Architecture = () => {
  const frame = useCurrentFrame();
  return (
    <SceneBase>
      <div style={{fontFamily: "\"Inter\", sans-serif", fontSize: 24, color: palette.violet, fontWeight: 700}}>
        Architecture
      </div>
      <div style={{...sectionTitle, marginTop: 14}}>Built for production.</div>
      <div style={{...sectionSub, marginTop: 16, maxWidth: 1500}}>
        Clean layered boundaries keep channels, orchestration, and infrastructure adapters independently evolvable.
      </div>
      <div style={{marginTop: 32, display: "flex", flexDirection: "column", gap: 14}}>
        {layers.map((layer, idx) => {
          const local = interpolate(frame, [idx * 12, idx * 12 + 28], [0, 1], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
          });
          return (
            <div
              key={layer}
              style={{
                ...card,
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "18px 24px",
                opacity: local,
                transform: `translateX(${interpolate(local, [0, 1], [64, 0])}px)`,
              }}
            >
              <div
                style={{
                  fontFamily: "\"Inter\", sans-serif",
                  fontSize: 40,
                  fontWeight: 700,
                  color: palette.text,
                }}
              >
                {layer}
              </div>
              <div
                style={{
                  width: 300,
                  height: 10,
                  borderRadius: 999,
                  background: "linear-gradient(90deg, #6366f1, #8b5cf6)",
                  opacity: Math.max(0.3, local),
                }}
              />
            </div>
          );
        })}
      </div>
    </SceneBase>
  );
};

const Cta = () => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const pulse = spring({
    frame,
    fps,
    config: {damping: 10, stiffness: 100},
  });

  return (
    <SceneBase>
      <div
        style={{
          ...card,
          flex: 1,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          textAlign: "center",
          transform: `scale(${0.95 + pulse * 0.05})`,
          padding: 48,
        }}
      >
        <Img
          src={staticFile("elephant-rounded.png")}
          style={{
            width: 170,
            height: 170,
            borderRadius: 40,
            marginBottom: 26,
            border: `1px solid ${palette.border}`,
            boxShadow: "0 18px 32px rgba(15, 23, 42, 0.12)",
          }}
        />
        <div style={{...sectionTitle, fontSize: 84}}>Ship faster with elephant.ai</div>
        <div style={{...sectionSub, marginTop: 18, maxWidth: 1240}}>
          Add to Lark. Capture context. Execute real workflows with full observability and safe approval gates.
        </div>
        <div
          style={{
            marginTop: 30,
            borderRadius: 999,
            padding: "14px 28px",
            fontFamily: "\"Inter\", sans-serif",
            fontSize: 34,
            fontWeight: 700,
            color: "#ffffff",
            background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
            boxShadow: "0 14px 30px rgba(99,102,241,0.28)",
          }}
        >
          github.com/cklxx/elephant.ai
        </div>
      </div>
    </SceneBase>
  );
};

export const ElephantPromo = () => {
  return (
    <AbsoluteFill style={{backgroundColor: palette.bg}}>
      <Sequence from={0} durationInFrames={150}>
        <Intro />
      </Sequence>
      <Sequence from={150} durationInFrames={170}>
        <Highlights />
      </Sequence>
      <Sequence from={320} durationInFrames={160}>
        <Architecture />
      </Sequence>
      <Sequence from={480} durationInFrames={120}>
        <Cta />
      </Sequence>
    </AbsoluteFill>
  );
};
