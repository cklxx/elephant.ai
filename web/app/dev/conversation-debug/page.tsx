import { redirect } from "next/navigation";

export default function LegacyConversationDebugPage() {
  redirect("/dev/diagnostics");
}
