import { cn } from "@/lib/utils";

interface Props {
  eyebrow?: string;
  title: React.ReactNode;
  description?: React.ReactNode;
  align?: "left" | "center";
  className?: string;
  action?: React.ReactNode;
  tone?: "light" | "dark";
}

export function SectionHeading({ eyebrow, title, description, align = "left", className, action, tone = "light" }: Props) {
  return (
    <div
      className={cn(
        "flex flex-col gap-4",
        align === "center" && "items-center text-center",
        action && "md:flex-row md:items-end md:justify-between",
        className,
      )}
    >
      <div className={cn("max-w-2xl", align === "center" && "mx-auto")}>
        {eyebrow && <p className={cn("label-caps mb-3", tone === "dark" ? "text-bronze" : "text-umber")}>{eyebrow}</p>}
        <h2 className={cn("headline-lg", tone === "dark" && "text-canvas")}>{title}</h2>
        {description && <p className={cn("mt-4 text-[15px] leading-7", tone === "dark" ? "text-canvas/70" : "text-umber")}>{description}</p>}
      </div>
      {action}
    </div>
  );
}
