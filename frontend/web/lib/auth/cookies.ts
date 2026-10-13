// Cookie names and helpers shared by the proxy, server actions and server components.
// Tokens live in httpOnly cookies, so browser scripts never see them (the Next server calls the API on the user's behalf).

export const ACCESS_COOKIE = "rovena_at";
export const REFRESH_COOKIE = "rovena_rt";

export interface TokenCookies {
  access_token: string;
  access_expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
}

export function cookieOptions(expiresAt: string) {
  const maxAge = Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000));
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge,
  };
}
