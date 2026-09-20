"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutDashboard, Users2, LogOut, Ticket, Briefcase, ListOrdered, FolderKanban, AlertTriangle, BarChart3, Settings as SettingsIcon, Archive, ScrollText, Sun, Moon } from "lucide-react";
import { NotificationBell } from "./notification-bell";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api/auth";
import type { User } from "@/lib/api/auth";
import { useTheme } from "@/app/theme-provider";

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/my-work", label: "My Work", icon: Briefcase },
  { href: "/priority-queue", label: "Priority Queue", icon: ListOrdered },
  { href: "/tickets", label: "Tickets", icon: Ticket },
  { href: "/projects", label: "Projects", icon: FolderKanban },
  { href: "/incidents", label: "Incidents", icon: AlertTriangle },
  { href: "/reports", label: "Reports", icon: BarChart3 },
  { href: "/administration/users", label: "Users", icon: Users2, section: "Administration" },
  { href: "/administration/backups", label: "Backup & Restore", icon: Archive, section: "Administration" },
  { href: "/administration/settings", label: "Settings", icon: SettingsIcon, section: "Administration" },
  { href: "/administration/audit-log", label: "Audit Log", icon: ScrollText, section: "Administration" },
];

export function Sidebar({ user }: { user: User }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { theme, toggle } = useTheme();

  const logout = useMutation({
    mutationFn: authApi.logout,
    onSuccess: () => {
      queryClient.setQueryData(["auth", "me"], null);
      queryClient.clear();
      router.push("/login");
    },
  });

  let lastSection: string | undefined;

  return (
    <aside className="flex h-screen w-64 flex-col border-r border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <div className="flex h-16 items-center justify-between gap-2 border-b border-slate-200 px-5 dark:border-slate-800">
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-600 text-sm font-bold text-white">
            IT
          </div>
          <span className="font-semibold text-slate-900 dark:text-slate-100">Operations Hub</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={toggle}
            className="flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800"
            aria-label="Toggle dark mode"
          >
            {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>
          <NotificationBell />
        </div>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        {navItems.map((item) => {
          const showHeader = item.section && item.section !== lastSection;
          lastSection = item.section;
          const active = pathname === item.href || pathname.startsWith(item.href + "/");
          const Icon = item.icon;
          return (
            <div key={item.href}>
              {showHeader && (
                <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                  {item.section}
                </div>
              )}
              <Link
                href={item.href}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-indigo-50 text-indigo-700 dark:bg-indigo-950 dark:text-indigo-400"
                    : "text-slate-600 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800"
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            </div>
          );
        })}
      </nav>

      <div className="border-t border-slate-200 p-3 dark:border-slate-800">
        <div className="mb-2 flex items-center gap-3 rounded-lg px-3 py-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-200 text-xs font-semibold text-slate-700 dark:bg-slate-700 dark:text-slate-200">
            {user.fullName
              .split(" ")
              .map((n) => n[0])
              .slice(0, 2)
              .join("")}
          </div>
          <div className="min-w-0">
            <p className="truncate text-sm font-medium text-slate-900 dark:text-slate-100">{user.fullName}</p>
            <p className="truncate text-xs text-slate-500 dark:text-slate-400">{user.roles.join(", ")}</p>
          </div>
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="w-full justify-start gap-2 text-slate-600 dark:text-slate-400"
          onClick={() => logout.mutate()}
          disabled={logout.isPending}
        >
          <LogOut className="h-4 w-4" />
          Log out
        </Button>
      </div>
    </aside>
  );
}
