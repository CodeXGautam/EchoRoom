"use client";

import { useEffect, useRef } from "react";
import type { Profile } from "@/lib/protocol";
import { RoomSocket, type SocketStatus } from "@/lib/roomSocket";

interface Props {
  profile: Profile;
  font: string;
  onStatus: (s: SocketStatus, detail?: string) => void;
  onPlayerCount: (n: number) => void;
}

/**
 * Hosts the Phaser game. Phaser touches `window` at import time, so it is
 * imported dynamically inside the effect — never during server rendering.
 */
export default function GameCanvas({ profile, font, onStatus, onPlayerCount }: Props) {
  const host = useRef<HTMLDivElement>(null);
  // Keep callbacks in refs so changing them doesn't restart the game.
  const cb = useRef({ onStatus, onPlayerCount });
  useEffect(() => {
    cb.current = { onStatus, onPlayerCount };
  });

  useEffect(() => {
    let cancelled = false;
    let game: import("phaser").Game | null = null;
    const socket = new RoomSocket(profile);
    const offStatus = socket.onStatus((s, d) => cb.current.onStatus(s, d));

    (async () => {
      const [Phaser, { RoomScene }] = await Promise.all([import("phaser"), import("@/game/RoomScene")]);
      await document.fonts.ready; // canvas text needs the web font loaded first
      if (cancelled || !host.current) return;

      game = new Phaser.Game({
        type: Phaser.AUTO,
        parent: host.current,
        backgroundColor: "#f6efe6",
        scale: { mode: Phaser.Scale.RESIZE, width: "100%", height: "100%" },
        render: { antialias: true },
        banner: false,
      });
      game.scene.add("room", RoomScene, true, {
        socket,
        font,
        onPlayerCount: (n: number) => cb.current.onPlayerCount(n),
      });
    })();

    // React StrictMode mounts → unmounts → mounts in dev; this must fully undo.
    return () => {
      cancelled = true;
      offStatus();
      socket.close();
      game?.destroy(true);
    };
  }, [profile, font]);

  return <div ref={host} className="absolute inset-0" />;
}
