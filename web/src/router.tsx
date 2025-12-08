import { Link as RouteLink, Outlet, Route, RootRoute, Router } from "@tanstack/solid-router";
import { type JSX } from "solid-js";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import ConsolePage from "@/routes/home";
import SessionsPage from "@/routes/sessions";

function Shell(props: { children: JSX.Element }) {
  return (
    <div class="mx-auto flex min-h-screen max-w-6xl flex-col px-4 py-6 lg:px-8">
      <header class="mb-6 flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <p class="text-xs uppercase tracking-wide text-muted-foreground">ALEX Console</p>
          <h1 class="text-2xl font-bold">TanStack Router + Solid</h1>
          <p class="text-sm text-muted-foreground">Shadcn-inspired console rebuilt for stability.</p>
        </div>
        <div class="flex items-center gap-2">
          <Badge variant="secondary">Solid</Badge>
          <Badge variant="secondary">TanStack Router</Badge>
          <Badge variant="secondary">UI · shadcn</Badge>
        </div>
      </header>
      {props.children}
      <footer class="mt-10 border-t pt-4 text-xs text-muted-foreground">
        Built for reliability after the Next.js outage — powered by Solid and TanStack Router.
      </footer>
    </div>
  );
}

function NavLink(props: { href: string; label: string; icon?: JSX.Element }) {
  return (
    <RouteLink
      to={props.href}
      class="inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors"
      activeProps={{ class: "bg-primary text-primary-foreground" }}
      inactiveProps={{ class: "text-muted-foreground hover:bg-muted" }}
    >
      {props.icon}
      <span>{props.label}</span>
    </RouteLink>
  );
}

function RootLayout() {
  return (
    <Shell>
      <div class="mb-6 flex flex-wrap items-center gap-2">
        <NavLink href="/" label="Console" />
        <NavLink href="/sessions" label="Sessions" />
        <a
          href="https://tanstack.com/router"
          target="_blank"
          rel="noreferrer"
          class="inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted"
        >
          Docs
        </a>
      </div>
      <Outlet />
    </Shell>
  );
}

const rootRoute = new RootRoute({ component: RootLayout });
const indexRoute = new Route({ getParentRoute: () => rootRoute, path: "/", component: ConsolePage });
const sessionsRoute = new Route({ getParentRoute: () => rootRoute, path: "/sessions", component: SessionsPage });

const routeTree = rootRoute.addChildren([indexRoute, sessionsRoute]);

export const router = new Router({ routeTree });

declare module "@tanstack/solid-router" {
  interface Register {
    router: typeof router;
  }
}
