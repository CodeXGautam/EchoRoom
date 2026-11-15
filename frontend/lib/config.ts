// NEXT_PUBLIC_* must be referenced literally so Next.js can inline them.
export const API_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
export const WS_URL = API_URL.replace(/^http/, "ws"); // http→ws, https→wss

export const AUTH_MODE: "dev" | "supabase" =
  process.env.NEXT_PUBLIC_AUTH_MODE === "supabase" ? "supabase" : "dev";

export const SUPABASE_URL = process.env.NEXT_PUBLIC_SUPABASE_URL ?? "";
export const SUPABASE_ANON_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "";

// Must match backend room.MaxSpeed, or the server will keep correcting you.
export const MAX_SPEED = 220;
