export default function Loading() {
  return (
    <div className="container-page flex min-h-[50vh] items-center justify-center">
      <div className="h-px w-24 overflow-hidden bg-travertine">
        <div className="h-px w-1/2 animate-[marquee_1.2s_ease-in-out_infinite] bg-ink" />
      </div>
    </div>
  );
}
