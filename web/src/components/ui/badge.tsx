import type { JSX } from "solid-js";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold", {
  variants: {
    variant: {
      default: "border-transparent bg-primary text-primary-foreground",
      secondary: "border-transparent bg-secondary text-secondary-foreground",
      outline: "text-foreground",
      success: "border-emerald-300 bg-emerald-50 text-emerald-800",
      warning: "border-amber-200 bg-amber-50 text-amber-800",
      destructive: "border-destructive/30 bg-destructive/10 text-destructive-foreground",
    },
  },
  defaultVariants: {
    variant: "default",
  },
});

export type BadgeProps = {
  class?: string;
  children?: JSX.Element;
} & VariantProps<typeof badgeVariants>;

export function Badge(props: BadgeProps) {
  return <span class={cn(badgeVariants({ variant: props.variant }), props.class)}>{props.children}</span>;
}
