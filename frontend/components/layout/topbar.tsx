"use client";

import { GlobalSearch } from "./global-search";

export function TopBar() {
  return (
    <div className="flex h-16 items-center border-b border-slate-200 bg-white px-8 dark:border-slate-800 dark:bg-slate-900">
      <GlobalSearch />
    </div>
  );
}
