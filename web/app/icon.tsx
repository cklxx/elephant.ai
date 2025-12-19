import { ImageResponse } from "next/og";

export const size = {
  width: 64,
  height: 64,
};

export const contentType = "image/png";
export const dynamic = "force-static";

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "linear-gradient(135deg, #e9edf5 0%, #dfe7f0 100%)",
        }}
      >
        <svg
          viewBox="0 0 64 64"
          role="img"
          aria-label="elephant.ai logo"
          style={{
            width: "80%",
            height: "80%",
          }}
        >
          <rect
            x="6"
            y="6"
            width="52"
            height="52"
            rx="14"
            fill="#f5f7fb"
            stroke="#cbd5e1"
            strokeWidth="2.5"
          />
          <ellipse
            cx="24"
            cy="34"
            rx="11"
            ry="13"
            fill="#dfe7f0"
            stroke="#0f172a"
            strokeWidth="2.25"
          />
          <circle cx="38" cy="30" r="12" fill="#f1f5f9" stroke="#0f172a" strokeWidth="2.25" />
          <path
            d="M44 34C52 38 52 48 44 52C36 56 30 50 34 44"
            fill="none"
            stroke="#0f172a"
            strokeWidth="3"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <circle cx="40" cy="28" r="1.5" fill="#0f172a" />
        </svg>
      </div>
    ),
    {
      ...size,
    }
  );
}
