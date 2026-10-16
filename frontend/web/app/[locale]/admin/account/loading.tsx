export default function AdminAccountLoading() {
  return (
    <div className="animate-pulse space-y-8" aria-busy="true">
      <div className="h-7 w-40 bg-travertine" />
      <div className="max-w-2xl space-y-px border border-travertine bg-travertine">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-14 bg-canvas" />
        ))}
      </div>
    </div>
  );
}
