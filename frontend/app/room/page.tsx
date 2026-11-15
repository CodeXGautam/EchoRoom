"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import type { SocketStatus } from "@/lib/roomSocket";
import { useSession, useSessionHydrated } from "@/store/session";
import { fredoka } from "../fonts";

const GameCanvas = dynamic(() => import("@/components/GameCanvas"), { ssr: false });

const SLOW_START_MS = 4000;

export default function RoomPage() {
  const router = useRouter();
  const { name, color } = useSession();
  const mounted = useSessionHydrated();
  const [status, setStatus] = useState<{ s: SocketStatus; detail?: string }>({ s: "connecting" });
  const [count, setCount] = useState(0);
  const [slow, setSlow] = useState(false);

  useEffect(() => {
    if (mounted && !name.trim()) router.replace("/");
  }, [mounted, name, router]);

  // A free Render instance takes ~1 min to wake. Say so instead of looking broken.
  // Only the first attempt reports "connecting", so this shows only on a cold start.
  useEffect(() => {
    const t = setTimeout(() => setSlow(true), SLOW_START_MS);
    return () => clearTimeout(t);
  }, []);

  // Stable identity: a new object would restart the game.
  const profile = useMemo(() => ({ name: name.trim(), avatar: { color } }), [name, color]);
  // next/font's generated family name, so canvas text uses the same font.
  const font = fredoka.style.fontFamily;

  if (!mounted || !name.trim()) return null;

  return (
    <main className="relative h-dvh w-full overflow-hidden">
      <GameCanvas
        profile={profile}
        font={font}
        onStatus={(s, detail) => setStatus({ s, detail })}
        onPlayerCount={setCount}
      />

      <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-2 p-3">
        <div className="sketch pointer-events-auto rounded-2xl bg-paper px-4 py-2">
          <Link href="/" className="font-semibold">
            EchoRoom
          </Link>
          <span className="text-ink/60"> · lobby · {count} online</span>
        </div>
        <StatusPill status={status.s} />
      </div>

      <p className="pointer-events-none absolute inset-x-0 bottom-3 text-center text-sm text-ink/60">
        WASD / arrow keys, or tap to walk · chat bubbles arrive in Phase 2
      </p>

      {status.s === "connecting" && slow && (
        <Overlay>
          Waking up the room server…
          <br />
          <span className="text-sm text-ink/60">Free hosting naps when idle. This can take up to a minute.</span>
        </Overlay>
      )}
      {status.s === "replaced" && (
        <Overlay>
          You opened EchoRoom in another tab.
          <br />
          <button className="mt-3 underline" onClick={() => location.reload()}>
            Use this tab instead
          </button>
        </Overlay>
      )}
      {status.s === "failed" && (
        <Overlay>
          Couldn&apos;t join: {status.detail}
          <br />
          <Link className="mt-3 inline-block underline" href="/">
            Back to the start
          </Link>
        </Overlay>
      )}
    </main>
  );
}

function StatusPill({ status }: { status: SocketStatus }) {
  const map: Record<SocketStatus, [string, string]> = {
    connecting: ["connecting", "bg-yellow-200"],
    open: ["connected", "bg-green-200"],
    reconnecting: ["reconnecting…", "bg-orange-200"],
    replaced: ["paused", "bg-zinc-200"],
    failed: ["error", "bg-red-200"],
    closed: ["closed", "bg-zinc-200"],
  };
  const [label, bg] = map[status];
  return <span className={`sketch rounded-full px-3 py-1 text-sm ${bg}`}>{label}</span>;
}

function Overlay({ children }: { children: React.ReactNode }) {
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-background/70 p-4">
      <div className="sketch max-w-sm rounded-2xl bg-paper p-6 text-center">{children}</div>
    </div>
  );
}
