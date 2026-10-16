export default function AdminProductEditLoading() {
  return (
    <div className="animate-pulse space-y-8" aria-busy="true">
      <div className="h-7 w-48 bg-travertine" />
      {[0, 1, 2].map((i) => (
        <div key={i} className="space-y-3 border border-travertine bg-paper p-6">
          <div className="h-3 w-24 bg-travertine" />
          <div className="h-10 bg-canvas" />
          <div className="h-10 bg-canvas" />
        </div>
      ))}
    </div>
  );
}
