"use client";

import type { OrderStatus } from "@/types";
import { statusTone } from "@/lib/mock/account";
import { useDict } from "@/lib/i18n/provider";
import { cn } from "@/lib/utils";

export function StatusBadge({ status }: { status: OrderStatus }) {
  const d = useDict();
  const tone = statusTone[status];
  return (
    <span
      className={cn(
        "chip",
        tone === "done" && "!border-success/40 !text-success",
        tone === "active" && "!border-bronze-deep/50 !text-bronze-deep",
        tone === "bad" && "!text-umber",
      )}
    >
      {d.status[status]}
    </span>
  );
}
