import { cn } from "@/lib/utils";

interface StatBlockProps {
  label: string;
  value: string | number;
  change?: string;
  variant?: "default" | "success" | "warning" | "danger";
}

const variantStyles = {
  default: "text-foreground",
  success: "text-green-600",
  warning: "text-amber-600",
  danger: "text-red-600",
};

export function StatBlock({ label, value, change, variant = "default" }: StatBlockProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className={cn("mt-1 text-2xl font-semibold", variantStyles[variant])}>
        {value}
      </p>
      {change && (
        <p className="mt-1 text-xs text-muted-foreground">{change}</p>
      )}
    </div>
  );
}
