"use client";

import { useState, useRef, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Search, Loader2, Ticket, AlertTriangle, MessageSquare } from "lucide-react";
import { searchApi } from "@/lib/api/search";
import { severityVariant } from "@/lib/ticket-display";
import { Badge } from "@/components/ui/badge";

export function GlobalSearch() {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [debounced, setDebounced] = useState("");
  const router = useRouter();
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, []);

  const { data: results, isFetching } = useQuery({
    queryKey: ["search", debounced],
    queryFn: () => searchApi.search(debounced),
    enabled: debounced.length >= 2,
  });

  const iconFor = (type: string) => {
    if (type === "incident") return <AlertTriangle className="h-3.5 w-3.5 text-orange-500" />;
    if (type === "comment") return <MessageSquare className="h-3.5 w-3.5 text-slate-400" />;
    return <Ticket className="h-3.5 w-3.5 text-slate-400" />;
  };

  const navigateTo = (r: NonNullable<typeof results>[number]) => {
    setOpen(false);
    setQuery("");
    if (r.type === "incident") {
      router.push(`/incidents/${r.entityId}`);
    } else {
      router.push(`/tickets/${r.entityId}`);
    }
  };

  return (
    <div ref={containerRef} className="relative w-80">
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
      <input
        placeholder="Search tickets, incidents, comments..."
        className="h-9 w-full rounded-lg border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm placeholder:text-slate-400 focus:border-indigo-400 focus:bg-white focus:outline-none focus:ring-2 focus:ring-indigo-100"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
        }}
        onFocus={() => query.length >= 2 && setOpen(true)}
      />

      {open && debounced.length >= 2 && (
        <div className="absolute left-0 top-11 z-30 max-h-96 w-[28rem] overflow-y-auto rounded-lg border border-slate-200 bg-white shadow-lg">
          {isFetching ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-4 w-4 animate-spin text-slate-300" />
            </div>
          ) : results && results.length > 0 ? (
            results.map((r) => (
              <button
                key={`${r.type}-${r.id}`}
                onClick={() => navigateTo(r)}
                className="flex w-full items-start gap-2 border-b border-slate-50 px-4 py-3 text-left last:border-0 hover:bg-slate-50"
              >
                {iconFor(r.type)}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-slate-400">{r.number}</span>
                    {r.severity && <Badge variant={severityVariant(r.severity as never)}>{r.severity}</Badge>}
                  </div>
                  <p className="truncate text-sm font-medium text-slate-800">{r.title}</p>
                  {r.type === "comment" && <p className="truncate text-xs text-slate-400">{r.snippet}</p>}
                </div>
              </button>
            ))
          ) : (
            <p className="px-4 py-6 text-center text-sm text-slate-400">No results.</p>
          )}
        </div>
      )}
    </div>
  );
}
