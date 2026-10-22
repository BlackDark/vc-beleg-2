import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import {
  CalendarDaysIcon,
  FileTextIcon,
  HomeIcon,
  LogOutIcon,
  ReceiptIcon,
  SettingsIcon,
  UserIcon,
  UtensilsIcon,
} from "lucide-react";
import type * as React from "react";
import { logoutMutation } from "@/api/gen/@tanstack/react-query.gen";
import { ReceiptCapture } from "@/components/receipt-capture";
import { Menu, MenuContent, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { sessionQuery, useUser } from "@/lib/session";
import { m } from "@/paraglide/messages";

type NavItem = {
  to: "/" | "/receipts" | "/month" | "/reports" | "/settings";
  label: () => string;
  icon: React.ComponentType<{ className?: string }>;
  exact?: boolean;
};

const nav: NavItem[] = [
  { to: "/", label: m.nav_home, icon: HomeIcon, exact: true },
  { to: "/receipts", label: m.nav_receipts, icon: ReceiptIcon },
  { to: "/month", label: m.nav_month, icon: CalendarDaysIcon },
  { to: "/reports", label: m.nav_reports, icon: FileTextIcon },
];

export function AppShell() {
  const user = useUser();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const logout = useMutation({
    ...logoutMutation(),
    onSuccess: (data) => {
      qc.clear();
      qc.setQueryData(sessionQuery.queryKey, data);
      navigate({ to: "/login" });
    },
  });

  return (
    <div className="min-h-dvh md:grid md:grid-cols-[15rem_1fr]">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:m-2 focus:rounded focus:bg-card focus:p-2"
      >
        {m.skip_to_content()}
      </a>
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-dvh flex-col border-r bg-card md:flex">
        <div className="flex items-center gap-2 px-5 py-5">
          <Logo />
          <span className="text-lg font-semibold tracking-tight">{m.app_name()}</span>
        </div>
        <nav className="grid gap-0.5 px-3" aria-label={m.nav_main()}>
          {nav.map((item) => (
            <Link
              key={item.to}
              to={item.to}
              activeOptions={{ exact: item.exact }}
              className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground data-[status=active]:bg-accent data-[status=active]:text-accent-foreground"
            >
              <item.icon className="size-4.5" />
              {item.label()}
            </Link>
          ))}
        </nav>
        <div className="px-3 pt-4">
          <ReceiptCapture variant="outline" className="w-full justify-start" />
        </div>
        <div className="mt-auto grid gap-0.5 border-t px-3 py-3">
          <Link
            to="/settings"
            className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted-foreground hover:bg-accent data-[status=active]:bg-accent"
          >
            <SettingsIcon className="size-4.5" />
            {m.nav_settings()}
          </Link>
          <button
            type="button"
            onClick={() => logout.mutate({})}
            className="flex items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-muted-foreground hover:bg-accent"
          >
            <LogOutIcon className="size-4.5" />
            {m.logout()}
          </button>
          <p className="truncate px-3 pt-2 text-xs text-muted-foreground">{user.name}</p>
        </div>
      </aside>

      {/* Mobile top bar */}
      <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b bg-card/95 px-4 pt-[env(safe-area-inset-top)] backdrop-blur md:hidden">
        <div className="flex items-center gap-2">
          <Logo />
          <span className="font-semibold">{m.app_name()}</span>
        </div>
        <Menu>
          <MenuTrigger className="rounded-full p-2 hover:bg-accent" aria-label={m.account_menu()}>
            <UserIcon className="size-5" />
          </MenuTrigger>
          <MenuContent>
            <div className="px-2.5 py-1.5 text-xs text-muted-foreground">{user.name}</div>
            <MenuItem onClick={() => navigate({ to: "/settings" })}>
              <SettingsIcon />
              {m.nav_settings()}
            </MenuItem>
            <MenuItem onClick={() => logout.mutate({})}>
              <LogOutIcon />
              {m.logout()}
            </MenuItem>
          </MenuContent>
        </Menu>
      </header>

      <main id="main" className="min-w-0 pb-24 md:pb-10">
        <div className="mx-auto w-full max-w-5xl px-4 py-5 sm:px-6 md:py-8">
          <Outlet />
        </div>
      </main>

      {/* Mobile bottom navigation */}
      <nav
        aria-label={m.nav_main()}
        className="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t bg-card/95 pb-[env(safe-area-inset-bottom)] backdrop-blur md:hidden"
      >
        {nav.slice(0, 2).map((item) => (
          <MobileNavLink key={item.to} item={item} />
        ))}
        <div className="flex items-center justify-center">
          <ReceiptCapture iconOnly />
        </div>
        {nav.slice(2).map((item) => (
          <MobileNavLink key={item.to} item={item} />
        ))}
      </nav>
    </div>
  );
}

function MobileNavLink({ item }: { item: NavItem }) {
  return (
    <Link
      to={item.to}
      activeOptions={{ exact: item.exact }}
      className="flex flex-col items-center gap-0.5 py-2 text-[11px] font-medium text-muted-foreground data-[status=active]:text-primary"
    >
      <item.icon className="size-5" />
      {item.label()}
    </Link>
  );
}

export function Logo() {
  return (
    <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground">
      <UtensilsIcon className="size-4.5" />
    </span>
  );
}
