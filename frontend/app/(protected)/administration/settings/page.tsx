"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Plus, Trash2 } from "lucide-react";

import { settingsApi } from "@/lib/api/settings";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

export default function SettingsPage() {
  const [newKey, setNewKey] = useState("");
  const [newValue, setNewValue] = useState("");
  const queryClient = useQueryClient();

  const { data: settings, isLoading } = useQuery({ queryKey: ["settings"], queryFn: settingsApi.list });

  const setMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => settingsApi.set(key, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      setNewKey("");
      setNewValue("");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (key: string) => settingsApi.delete(key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["settings"] }),
  });

  return (
    <div className="mx-auto max-w-3xl px-8 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Settings</h1>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">Application-wide key-value settings.</p>
      </div>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">Add / Update Setting</CardTitle>
          <CardDescription>Setting an existing key overwrites its value.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-end gap-3">
            <div className="flex-1 space-y-1.5">
              <Label htmlFor="key">Key</Label>
              <Input id="key" placeholder="companyName" value={newKey} onChange={(e) => setNewKey(e.target.value)} />
            </div>
            <div className="flex-1 space-y-1.5">
              <Label htmlFor="value">Value</Label>
              <Input id="value" value={newValue} onChange={(e) => setNewValue(e.target.value)} />
            </div>
            <Button
              onClick={() => setMutation.mutate({ key: newKey, value: newValue })}
              disabled={!newKey.trim() || setMutation.isPending}
            >
              {setMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
            </Button>
          </div>
        </CardContent>
      </Card>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : settings && settings.length > 0 ? (
        <Card>
          <CardContent className="p-0">
            {settings.map((s) => (
              <div
                key={s.key}
                className="flex items-center justify-between border-b border-slate-100 px-5 py-3 last:border-0 dark:border-slate-800"
              >
                <div>
                  <p className="font-mono text-sm text-slate-800 dark:text-slate-200">{s.key}</p>
                  <p className="text-sm text-slate-500 dark:text-slate-400">{s.value}</p>
                </div>
                <button
                  onClick={() => deleteMutation.mutate(s.key)}
                  className="text-slate-400 hover:text-red-600"
                  disabled={deleteMutation.isPending}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : (
        <p className="rounded-lg border border-dashed border-slate-200 py-12 text-center text-sm text-slate-400 dark:border-slate-800">
          No settings configured yet.
        </p>
      )}
    </div>
  );
}
