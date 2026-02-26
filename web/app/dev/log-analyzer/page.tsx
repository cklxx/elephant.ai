import { redirect } from "next/navigation";

export default function LogAnalyzerPage() {
  redirect("/dev/diagnostics#structured-log-analyzer");
}
