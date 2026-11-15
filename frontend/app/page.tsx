"use client";

import { useRouter } from "next/navigation";
import DoodlePreview from "@/components/DoodlePreview";
import { DOODLE_COLORS, useSession, useSessionHydrated } from "@/store/session";

export default function Home() {
  const router = useRouter();
  const { name, color, setName, setColor } = useSession();
  // The store is restored from localStorage only in the browser; render the
  // form after that so server and client HTML match.
  const mounted = useSessionHydrated();

  const trimmed = name.trim();
  const valid = trimmed.length >= 1 && [...trimmed].length <= 20;

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
              if (!valid) return;
              setName(trimmed);
              router.push("/room");
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

            <button
              type="submit"
              disabled={!valid}
              className="sketch w-full rounded-2xl bg-[#ffd6a5] py-3 text-lg font-semibold transition-transform active:translate-x-[2px] active:translate-y-[2px] active:shadow-none disabled:opacity-50"
            >
              Enter the lobby
            </button>
          </form>
        )}
      </div>
    </main>
  );
}
