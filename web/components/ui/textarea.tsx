"use client";

import * as React from "react";

import { cn } from "@/lib/utils";
import { baseFormControlClasses } from "@/components/ui/form-control";

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>;

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, ...props }, ref) => {
    return (
      <textarea
        className={cn(
          "flex min-h-[96px] w-full resize-none rounded-lg border border-input bg-background px-3 py-2 text-sm transition",
          baseFormControlClasses,
          className,
        )}
        ref={ref}
        {...props}
      />
    );
  }
);
Textarea.displayName = "Textarea";

export { Textarea };
