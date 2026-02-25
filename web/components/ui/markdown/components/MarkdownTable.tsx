import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function MarkdownTable({ className, ...props }: HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="my-4 overflow-x-auto">
      <table
        className={cn("w-full border-collapse border border-border", className)}
        {...props}
      />
    </div>
  );
}

export function MarkdownTableHead({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn("bg-muted/80", className)} {...props} />;
}

export function MarkdownTableBody({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return (
    <tbody className={cn("divide-y divide-border bg-muted/40", className)} {...props} />
  );
}

export function MarkdownTableRow({ className, ...props }: HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={cn("border-border border-b", className)} {...props} />;
}

export function MarkdownTableHeaderCell({ className, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={cn(
        "whitespace-nowrap px-4 py-2 text-left text-sm font-semibold",
        className,
      )}
      {...props}
    />
  );
}

export function MarkdownTableCell({ className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn("px-4 py-2 align-top text-sm", className)} {...props} />;
}
