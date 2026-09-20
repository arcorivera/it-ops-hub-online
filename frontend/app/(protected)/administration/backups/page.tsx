"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Archive, AlertTriangle, RotateCcw } from "lucide-react";

import { backupsApi } from "@/lib/api/backups";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function BackupsPage() {
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [restoreMessage, setRestoreMessage] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const { data: backups, isLoading } = useQuery({ queryKey: ["backups"], queryFn: backupsApi.list });

  const createMutation = useMutation({
    mutationFn: backupsApi.create,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["backups"] }),
  });

  const restoreMutation = useMutation({
    mutationFn: (id: string) => backupsApi.restore(id),
    onSuccess: (result) => {
      setRestoreMessage(result.message);
      setConfirmId(null);
    },
  });

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Backup &amp; Restore</h1>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Real SQLite snapshots. Restoring always creates a safety backup first.
          </p>
        </div>
        <Button onClick={() => createMutation.mutate()} disabled={createMutation.isPending}>
          {createMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Archive className="h-4 w-4" />}
          Create Backup
        </Button>
      </div>

      {restoreMessage && (
        <Card className="mb-4 border-amber-300 dark:border-amber-700">
          <CardContent className="flex items-start gap-3 p-4">
            <AlertTriangle className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-500" />
            <p className="text-sm text-amber-800 dark:text-amber-300">{restoreMessage}</p>
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : backups && backups.length > 0 ? (
        <Card>
          <CardContent className="p-0">
            {backups.map((b) => (
              <div
                key={b.id}
                className="flex items-center justify-between border-b border-slate-100 px-5 py-3 last:border-0 dark:border-slate-800"
              >
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm text-slate-700 dark:text-slate-300">{b.fileName}</span>
                    {b.isSafety && <Badge variant="purple">Safety backup</Badge>}
                  </div>
                  <p className="text-xs text-slate-400">
                    {(b.sizeBytes / 1024).toFixed(1)} KB · {new Date(b.createdAt).toLocaleString()}
                  </p>
                </div>

                {confirmId === b.id ? (
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-red-600">Restart required after restore. Confirm?</span>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => restoreMutation.mutate(b.id)}
                      disabled={restoreMutation.isPending}
                    >
                      {restoreMutation.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : "Yes, Restore"}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => setConfirmId(null)}>
                      Cancel
                    </Button>
                  </div>
                ) : (
                  <Button size="sm" variant="outline" onClick={() => setConfirmId(b.id)}>
                    <RotateCcw className="h-3.5 w-3.5" />
                    Restore
                  </Button>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      ) : (
        <p className="rounded-lg border border-dashed border-slate-200 py-12 text-center text-sm text-slate-400 dark:border-slate-800">
          No backups yet.
        </p>
      )}
    </div>
  );
}
