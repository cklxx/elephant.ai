import type { JSX } from "solid-js";
import { cn } from "@/lib/utils";

export function Card(props: { class?: string; children?: JSX.Element }) {
  return (
    <div class={cn("rounded-md border bg-card text-card-foreground shadow", props.class)}>
      {props.children}
    </div>
  );
}

export function CardHeader(props: { class?: string; children?: JSX.Element }) {
  return <div class={cn("flex flex-col space-y-1.5 p-4", props.class)}>{props.children}</div>;
}

export function CardTitle(props: { class?: string; children?: JSX.Element }) {
  return <h3 class={cn("text-lg font-semibold leading-none tracking-tight", props.class)}>{props.children}</h3>;
}

export function CardDescription(props: { class?: string; children?: JSX.Element }) {
  return <p class={cn("text-sm text-muted-foreground", props.class)}>{props.children}</p>;
}

export function CardContent(props: { class?: string; children?: JSX.Element }) {
  return <div class={cn("p-4 pt-0", props.class)}>{props.children}</div>;
}
