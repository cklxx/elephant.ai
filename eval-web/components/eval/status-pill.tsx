import { cn } from "@/lib/utils";

interface StatusPillProps {
  status: string;
}

const statusStyles: Record<string, string> = {
  completed: "bg-green-100 text-green-800",
  running: "bg-blue-100 text-blue-800",
  pending: "bg-amber-100 text-amber-800",
  failed: "bg-red-100 text-red-800",
  active: "bg-green-100 text-green-800",
  archived: "bg-gray-100 text-gray-600",
  draft: "bg-amber-100 text-amber-800",
  gold: "bg-yellow-100 text-yellow-800",
  silver: "bg-gray-100 text-gray-700",
  bronze: "bg-orange-100 text-orange-800",
  reject: "bg-red-100 text-red-800",
};

export function StatusPill({ status }: StatusPillProps) {
  const style = statusStyles[status] ?? "bg-gray-100 text-gray-600";
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        style,
      )}
    >
      {status}
    </span>
  );
}
