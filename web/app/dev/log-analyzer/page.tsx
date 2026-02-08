import { redirect } from "next/navigation";

export default function LegacyLogAnalyzerPage() {
  redirect("/dev/diagnostics");
}
