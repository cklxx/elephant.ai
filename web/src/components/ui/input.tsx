import { type JSX, splitProps } from "solid-js";
import { cn } from "@/lib/utils";

export type InputProps = {
  class?: string;
} & JSX.InputHTMLAttributes<HTMLInputElement>;

export function Input(props: InputProps) {
  const [local, rest] = splitProps(props, ["class"]);
  return (
    <input
      class={cn(
        "flex h-10 w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm transition-colors",
        "file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:outline-none focus-visible:ring-2",
        "focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 ring-offset-background",
        local.class,
      )}
      {...rest}
    />
  );
}
