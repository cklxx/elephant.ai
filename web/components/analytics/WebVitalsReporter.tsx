"use client";

import { useReportWebVitals } from "next/web-vitals";
import type { Metric } from "web-vitals";
import { buildApiUrl } from "@/lib/api-base";

function sendMetric(metric: Metric) {
  if (typeof window === "undefined") {
    return;
  }

  const payload = {
    name: metric.name,
    value: metric.value,
    delta: metric.delta,
    id: metric.id,
    rating: metric.rating,
    navigation_type: metric.navigationType,
    page: window.location?.pathname ?? "/",
    ts: Date.now(),
  };

  const body = JSON.stringify(payload);
  const endpoint = buildApiUrl("/api/metrics/web-vitals");
  const blob = new Blob([body], { type: "application/json" });

  if (navigator.sendBeacon) {
    navigator.sendBeacon(endpoint, blob);
    return;
  }

  fetch(endpoint, {
    method: "POST",
    body,
    headers: { "Content-Type": "application/json" },
    keepalive: true,
    credentials: "omit",
  }).catch(() => {
    // Silently ignore failures to avoid impacting UX
  });
}

export function WebVitalsReporter() {
  useReportWebVitals(sendMetric);
  return null;
}

