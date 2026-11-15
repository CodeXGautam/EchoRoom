import { createClient, type SupabaseClient } from "@supabase/supabase-js";
import { AUTH_MODE, SUPABASE_ANON_KEY, SUPABASE_URL, TURNSTILE_SITE_KEY } from "./config";

let client: SupabaseClient | null = null;

function supabase(): SupabaseClient {
  if (!SUPABASE_URL || !SUPABASE_ANON_KEY) {
    throw new Error("NEXT_PUBLIC_SUPABASE_URL / NEXT_PUBLIC_SUPABASE_ANON_KEY are not set");
  }
  client ??= createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
  return client;
}

/** Thrown when there is no session and one can't be created silently. */
export class SignInRequiredError extends Error {
  constructor() {
    super("Please sign in again");
  }
}

/** True when a CAPTCHA token is needed before creating a guest. */
export const CAPTCHA_ENABLED = AUTH_MODE === "supabase" && TURNSTILE_SITE_KEY !== "";

/** Whether this browser already has a (refreshable) session. */
export async function hasSession(): Promise<boolean> {
  if (AUTH_MODE === "dev") return true;
  const { data } = await supabase().auth.getSession();
  return data.session !== null;
}

/**
 * Creates an anonymous Supabase user. With CAPTCHA protection on in Supabase,
 * captchaToken is required (single-use, from the Turnstile widget). Only
 * needed once per browser: the session is then kept in localStorage and
 * refreshed automatically.
 */
export async function signInAsGuest(captchaToken?: string): Promise<void> {
  if (AUTH_MODE === "dev") return;
  const { data, error } = await supabase().auth.signInAnonymously({ options: { captchaToken } });
  if (error || !data.session) {
    throw new Error(`Guest sign-in failed: ${error?.message ?? "no session"}`);
  }
}

/**
 * Returns a bearer token for the Go API.
 *
 * dev:      a random id per browser tab (sessionStorage), so two tabs = two guests.
 * supabase: the current session's JWT. All tabs share it, so opening a second
 *           tab moves your doodle there.
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

  const { data } = await supabase().auth.getSession(); // refreshes an expired token
  if (data.session) return data.session.access_token;

  // No session: without CAPTCHA we can create one silently, with it the
  // user must go back to the landing page and pass the widget.
  if (CAPTCHA_ENABLED) throw new SignInRequiredError();
  await signInAsGuest();
  const again = await supabase().auth.getSession();
  if (!again.data.session) throw new SignInRequiredError();
  return again.data.session.access_token;
}
