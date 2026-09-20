import type { Severity, Priority, TicketStatus } from "@/lib/api/tickets";

export function severityVariant(s: Severity): "red" | "orange" | "amber" | "slate" {
  switch (s) {
    case "S1":
      return "red";
    case "S2":
      return "orange";
    case "S3":
      return "amber";
    default:
      return "slate";
  }
}

export function priorityVariant(p: Priority): "red" | "orange" | "amber" | "slate" {
  switch (p) {
    case "Critical":
      return "red";
    case "High":
      return "orange";
    case "Medium":
      return "amber";
    default:
      return "slate";
  }
}

export function statusVariant(
  s: TicketStatus
): "slate" | "blue" | "amber" | "purple" | "emerald" | "red" {
  if (s === "CLOSED" || s === "CANCELLED") return "slate";
  if (s === "RESOLVED" || s === "UAT_PASSED" || s === "PRE_PROD_PASSED") return "emerald";
  if (s === "UAT_FAILED" || s === "PRE_PROD_FAILED" || s === "PRODUCTION_FAILED") return "red";
  if (s === "PENDING") return "purple";
  if (s === "NEW" || s === "ACKNOWLEDGED" || s === "ASSIGNED") return "blue";
  return "amber"; // IN_PROGRESS, FOR_UAT, FOR_PRE_PROD, FOR_PRODUCTION
}
