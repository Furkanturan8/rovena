export default function AdminStockLoading() {
  return (
    <div className="animate-pulse space-y-8" aria-busy="true">
      <div className="h-7 w-40 bg-travertine" />
      <div className="h-12 w-full max-w-xl bg-travertine" />
      <div className="space-y-px border border-travertine bg-travertine">
        {[0, 1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="h-12 bg-canvas" />
        ))}
      </div>
    </div>
  );
}
