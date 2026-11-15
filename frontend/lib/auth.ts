import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { AUTH_MODE, SUPABASE_ANON_KEY, SUPABASE_URL } from "./config";

let client: SupabaseClient | null = null;

function supabase(): SupabaseClient {
  if (!SUPABASE_URL || !SUPABASE_ANON_KEY) {
    throw new Error("NEXT_PUBLIC_SUPABASE_URL / NEXT_PUBLIC_SUPABASE_ANON_KEY are not set");
  }
  client ??= createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
  return client;
}

/**
 * Returns a bearer token for the Go API.
 *
 * dev:      a random id per browser tab (sessionStorage), so two tabs = two guests.
 * supabase: the current session's JWT, signing in anonymously the first time.
 *           Supabase keeps the session in localStorage, so all tabs share one
 *           user — opening a second tab moves your doodle there.
 */
export async function getAccessToken(): Promise<string> {
  if (AUTH_MODE === "dev") {
    let id = sessionStorage.getItem("echoroom.devId");
    if (!id) {
      id = crypto.randomUUID();
      sessionStorage.setItem("echoroom.devId", id);
    }
    return `dev:${id}`;
  }

  const sb = supabase();
  const { data } = await sb.auth.getSession(); // refreshes an expired token
  if (data.session) return data.session.access_token;

  const { data: anon, error } = await sb.auth.signInAnonymously();
  if (error || !anon.session) {
    throw new Error(`Anonymous sign-in failed: ${error?.message ?? "no session"}`);
  }
  return anon.session.access_token;
}
