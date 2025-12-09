import { ImageResponse } from "next/og";

export const size = {
  width: 64,
  height: 64,
};

export const contentType = "image/png";
export const runtime = "edge";
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
          aria-label="Spinner logo"
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
          <path
            d="M20 36c0-6.627 5.373-12 12-12 3.314 0 6 2.686 6 6s-2.686 6-6 6c-3.333 0-6-2.667-6-6"
            fill="none"
            stroke="#0f172a"
            strokeWidth="2.25"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <circle cx="18" cy="30" r="4" fill="#d9e3ec" stroke="#0f172a" strokeWidth="2" />
          <circle cx="46" cy="24" r="4" fill="#dfe7f0" stroke="#0f172a" strokeWidth="2" />
          <circle cx="42" cy="44" r="4" fill="#c7d2fe" stroke="#0f172a" strokeWidth="2" />
          <circle cx="26" cy="46" r="4" fill="#cbd5e1" stroke="#0f172a" strokeWidth="2" />
          <path
            d="M22 32l6-4m10-2 4-2m-2 20-4-6m-8 10-4-6"
            stroke="#64748b"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
      </div>
    ),
    {
      ...size,
    }
  );
}
