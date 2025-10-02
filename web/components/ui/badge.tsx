import * as React from "react"
import { cn } from "@/lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "warning" | "error" | "info"
}

function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors",
        {
          "bg-gray-100 text-gray-900": variant === "default",
          "bg-green-100 text-green-900": variant === "success",
          "bg-yellow-100 text-yellow-900": variant === "warning",
          "bg-red-100 text-red-900": variant === "error",
          "bg-blue-100 text-blue-900": variant === "info",
        },
        className
      )}
      {...props}
    />
  )
}

export { Badge }
