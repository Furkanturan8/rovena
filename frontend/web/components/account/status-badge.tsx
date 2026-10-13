"use client";

import type { DisplayStatus } from "@/lib/order-status";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

const tone: Record<DisplayStatus, "neutral" | "active" | "done" | "bad"> = {
  pending: "neutral",
  awaiting_payment: "neutral",
  paid: "active",
  confirmed: "done",
  cancelled: "bad",
  refunded: "bad",
};

export function StatusBadge({ status }: { status: DisplayStatus }) {
  const d = useDict();
  const t = tone[status];
  return (
    <span
      className={cn(
        "chip",
        t === "done" && "!border-success/40 !text-success",
        t === "active" && "!border-bronze-deep/50 !text-bronze-deep",
        t === "bad" && "!text-umber",
      )}
    >
      {d.status[status]}
    </span>
  );
}
