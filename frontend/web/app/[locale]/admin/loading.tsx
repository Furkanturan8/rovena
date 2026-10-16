export default function AdminLoading() {
  return (
    <div className="grid animate-pulse gap-px border border-travertine bg-travertine sm:grid-cols-2" aria-busy="true">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="bg-canvas p-6">
          <div className="h-8 w-16 bg-travertine" />
          <div className="mt-3 h-3 w-28 bg-travertine" />
        </div>
      ))}
    </div>
  );
}
