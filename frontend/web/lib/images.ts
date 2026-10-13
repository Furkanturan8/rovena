/**
 * Medya adreslerini çözer. NEXT_PUBLIC_MEDIA_URL tanımlıysa (ör. Supabase Storage public bucket)
 * "/media/..." yolları doğrudan o adrese çevrilir; tanımlı değilse yol aynen kalır ve
 * public/media altından servis edilir.
 */
export function resolveMediaUrl(path: string): string {
  const base = process.env.NEXT_PUBLIC_MEDIA_URL?.trim().replace(/\/+$/, "");
  if (!base || !path.startsWith("/media/")) return path;
  const dest = base.endsWith("/media") ? base : `${base}/media`;
  return `${dest}${path.slice("/media".length)}`;
}
