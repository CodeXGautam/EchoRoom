// NEXT_PUBLIC_* must be referenced literally so Next.js can inline them.
export const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
export const WS_URL = API_URL.replace(/^http/, "ws"); // http→ws, https→wss

export const AUTH_MODE: "dev" | "supabase" =
  process.env.NEXT_PUBLIC_AUTH_MODE === "supabase" ? "supabase" : "dev";

/**
 * Must be the bare project URL (https://<ref>.supabase.co). supabase-js
 * appends /auth/v1/... itself, so a pasted REST URL (…/rest/v1/) silently
 * breaks every auth call with a 404. Keep only scheme + host.
 */
function projectOrigin(raw: string | undefined): string {
  if (!raw) return "";
  try {
    const u = new URL(raw);
    if (u.pathname !== "/" && u.pathname !== "") {
      console.warn(`NEXT_PUBLIC_SUPABASE_URL should be ${u.origin} (ignoring path "${u.pathname}")`);
    }
    return u.origin;
  } catch {
    console.error(`NEXT_PUBLIC_SUPABASE_URL is not a valid URL: ${raw}`);
    return "";
  }
}

export const SUPABASE_URL = projectOrigin(process.env.NEXT_PUBLIC_SUPABASE_URL);
export const SUPABASE_ANON_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "";

// Must match backend room.MaxSpeed, or the server will keep correcting you.
export const MAX_SPEED = 220;
