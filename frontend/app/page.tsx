"use client";

import { Turnstile, type TurnstileInstance } from "@marsidev/react-turnstile";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import DoodlePreview from "@/components/DoodlePreview";
import { CAPTCHA_ENABLED, hasSession, signInAsGuest } from "@/lib/auth";
import { TURNSTILE_SITE_KEY } from "@/lib/config";
import { DOODLE_COLORS, useSession, useSessionHydrated } from "@/store/session";

export default function Home() {
  const router = useRouter();
  const { name, color, setName, setColor } = useSession();
  // The store is restored from localStorage only in the browser; render the
  // form after that so server and client HTML match.
  const mounted = useSessionHydrated();

  // CAPTCHA is only needed to create a guest; a returning browser already has
  // a session. null = still checking.
  const [needsCaptcha, setNeedsCaptcha] = useState<boolean | null>(CAPTCHA_ENABLED ? null : false);
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const turnstile = useRef<TurnstileInstance>(undefined);

  useEffect(() => {
    if (!CAPTCHA_ENABLED) return;
    hasSession()
      .then((has) => setNeedsCaptcha(!has))
      .catch(() => setNeedsCaptcha(true));
  }, []);

  const trimmed = name.trim();
  const valid = trimmed.length >= 1 && [...trimmed].length <= 20;
  const ready = valid && needsCaptcha !== null && (!needsCaptcha || captchaToken !== null) && !busy;

  async function enter() {
    if (!ready) return;
    setName(trimmed);
    setError(null);
    if (needsCaptcha) {
      setBusy(true);
      try {
        await signInAsGuest(captchaToken!);
      } catch (e) {
        // Tokens are single-use: get a fresh one before the next attempt.
        setCaptchaToken(null);
        turnstile.current?.reset();
        setError(e instanceof Error ? e.message : String(e));
        setBusy(false);
        return;
      }
    }
    router.push("/room");
  }

  return (
    <main className="flex flex-1 items-center justify-center px-4 py-10">
      <div className="sketch w-full max-w-md rounded-3xl bg-paper p-8">
        <h1 className="text-center text-4xl font-semibold tracking-tight">EchoRoom</h1>
        <p className="mt-1 text-center text-ink/70">wander around, say hi, watch words float</p>

        <div className="mt-6 flex justify-center">
          <DoodlePreview color={mounted ? color : DOODLE_COLORS[0]} />
        </div>

        {mounted && (
          <form
            className="mt-6 space-y-5"
            onSubmit={(e) => {
              e.preventDefault();
              void enter();
            }}
          >
            <label className="block">
              <span className="text-sm font-medium">Your name</span>
              <input
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={20}
                placeholder="e.g. Pickle"
                className="mt-1 w-full rounded-xl border-[3px] border-ink bg-white px-4 py-2 text-lg outline-none focus:bg-yellow-50"
              />
            </label>

            <fieldset>
              <legend className="text-sm font-medium">Pick a colour</legend>
              <div className="mt-2 flex flex-wrap gap-2">
                {DOODLE_COLORS.map((c) => (
                  <button
                    key={c}
                    type="button"
                    aria-label={`colour ${c}`}
                    aria-pressed={c === color}
                    onClick={() => setColor(c)}
                    className={`h-9 w-9 rounded-full border-[3px] border-ink transition-transform ${
                      c === color ? "scale-110 shadow-[2px_2px_0_var(--ink)]" : "hover:scale-105"
                    }`}
                    style={{ background: c }}
                  />
                ))}
              </div>
            </fieldset>

            {needsCaptcha && (
              <Turnstile
                ref={turnstile}
                siteKey={TURNSTILE_SITE_KEY}
                options={{ size: "flexible", theme: "light" }}
                onSuccess={setCaptchaToken}
                onExpire={() => setCaptchaToken(null)}
                onError={() => setCaptchaToken(null)}
              />
            )}

            {error && <p className="text-sm text-red-700">{error}</p>}

            <button
              type="submit"
              disabled={!ready}
              className="sketch w-full rounded-2xl bg-[#ffd6a5] py-3 text-lg font-semibold transition-transform active:translate-x-[2px] active:translate-y-[2px] active:shadow-none disabled:opacity-50"
            >
              {busy ? "Joining…" : "Enter the lobby"}
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
