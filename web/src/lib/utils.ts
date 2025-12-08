import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatTimestamp(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function toRelativeTime(value?: string | null): string {
  if (!value) return "";
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const diffSeconds = (date.getTime() - Date.now()) / 1000;
  const divisions = [
    { amount: 60, unit: "second" },
    { amount: 60, unit: "minute" },
    { amount: 24, unit: "hour" },
    { amount: 7, unit: "day" },
    { amount: 4.34524, unit: "week" },
    { amount: 12, unit: "month" },
    { amount: Number.POSITIVE_INFINITY, unit: "year" },
  ] as const;

  let duration = diffSeconds;
  for (const division of divisions) {
    if (Math.abs(duration) < division.amount) {
      return formatter.format(Math.round(duration), division.unit as Intl.RelativeTimeFormatUnit);
    }
    duration /= division.amount;
  }
  return formatter.format(Math.round(duration), "year");
}
