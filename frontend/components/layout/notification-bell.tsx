"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Bell, Loader2 } from "lucide-react";
import { notificationsApi } from "@/lib/api/notifications";
import { cn } from "@/lib/utils";

export function NotificationBell() {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const queryClient = useQueryClient();

  const { data: unread } = useQuery({
    queryKey: ["notifications", "unread-count"],
    queryFn: notificationsApi.unreadCount,
    refetchInterval: 30_000, // real polling — reflects background engine activity
  });

  const { data: list, isLoading } = useQuery({
    queryKey: ["notifications", "list"],
    queryFn: () => notificationsApi.list(),
    enabled: open,
  });

  const markReadMutation = useMutation({
    mutationFn: notificationsApi.markRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });

  const markAllReadMutation = useMutation({
    mutationFn: notificationsApi.markAllRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notifications"] });
    },
  });

  const count = unread?.count ?? 0;

  return (
    <div className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className="relative flex h-9 w-9 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100"
      >
        <Bell className="h-4.5 w-4.5" />
        {count > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-semibold text-white">
            {count > 99 ? "99+" : count}
          </span>
        )}
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 top-11 z-20 w-80 rounded-lg border border-slate-200 bg-white shadow-lg">
            <div className="flex items-center justify-between border-b border-slate-100 px-4 py-2.5">
              <span className="text-sm font-semibold text-slate-900">Notifications</span>
              {count > 0 && (
                <button
                  onClick={() => markAllReadMutation.mutate()}
                  className="text-xs font-medium text-indigo-600 hover:underline"
                >
                  Mark all read
                </button>
              )}
            </div>
            <div className="max-h-96 overflow-y-auto">
              {isLoading ? (
                <div className="flex justify-center py-8">
                  <Loader2 className="h-4 w-4 animate-spin text-slate-300" />
                </div>
              ) : list && list.length > 0 ? (
                list.map((n) => (
                  <button
                    key={n.id}
                    onClick={() => {
                      markReadMutation.mutate(n.id);
                      setOpen(false);
                      if (n.entityType === "ticket") router.push(`/tickets/${n.entityId}`);
                    }}
                    className={cn(
                      "block w-full border-b border-slate-50 px-4 py-3 text-left last:border-0 hover:bg-slate-50",
                      !n.isRead && "bg-indigo-50/50"
                    )}
                  >
                    <div className="flex items-start gap-2">
                      {!n.isRead && <span className="mt-1.5 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-indigo-600" />}
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-slate-900">{n.title}</p>
                        {n.message && <p className="truncate text-xs text-slate-500">{n.message}</p>}
                        <p className="mt-0.5 text-[11px] text-slate-400">
                          {new Date(n.createdAt).toLocaleString()}
                        </p>
                      </div>
                    </div>
                  </button>
                ))
              ) : (
                <p className="px-4 py-8 text-center text-sm text-slate-400">No notifications yet.</p>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}
