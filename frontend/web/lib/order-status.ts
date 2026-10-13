import type { ApiOrderStatus } from "@/lib/api/types";

export type DisplayStatus = ApiOrderStatus | "refunded";

/** A cancelled order whose payment was returned reads as "refunded". */
export function displayStatus(status: ApiOrderStatus, refund?: "pending" | "refunded"): DisplayStatus {
  return status === "cancelled" && refund === "refunded" ? "refunded" : status;
}
