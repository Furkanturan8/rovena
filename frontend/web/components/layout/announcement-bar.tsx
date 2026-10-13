export function AnnouncementBar({ messages }: { messages: string[] }) {
  const row = [...messages, ...messages];
  return (
    <div className="overflow-hidden bg-ink py-2.5 text-canvas">
      <div className="animate-marquee flex w-max gap-16 whitespace-nowrap">
        {[...row, ...row].map((m, i) => (
          <span key={i} className="label-caps flex items-center gap-16 text-[10px] text-canvas/80">
            {m}
            <span aria-hidden className="text-bronze">◆</span>
          </span>
        ))}
      </div>
    </div>
  );
}
