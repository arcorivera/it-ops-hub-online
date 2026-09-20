import * as React from "react";
import { cn } from "@/lib/utils";

export function Badge({
  className,
  variant = "default",
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & {
  variant?: "default" | "red" | "orange" | "amber" | "emerald" | "blue" | "slate" | "purple";
}) {
  const variants: Record<string, string> = {
    default: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
    red: "bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-400",
    orange: "bg-orange-100 text-orange-700 dark:bg-orange-950 dark:text-orange-400",
    amber: "bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400",
    emerald: "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400",
    blue: "bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-400",
    slate: "bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-400",
    purple: "bg-purple-100 text-purple-700 dark:bg-purple-950 dark:text-purple-400",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
        variants[variant],
        className
      )}
      {...props}
    />
  );
}
