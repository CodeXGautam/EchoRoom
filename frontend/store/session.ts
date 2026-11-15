import { useSyncExternalStore } from "react";
import { create } from "zustand";
import { persist } from "zustand/middleware";

// Soft pastel doodle colours. The backend accepts any #rrggbb.
export const DOODLE_COLORS = [
  "#ffb3c6", // bubblegum
  "#ffd6a5", // peach
  "#fdffb6", // butter
  "#caffbf", // mint
  "#9bf6ff", // sky
  "#a0c4ff", // periwinkle
  "#bdb2ff", // lavender
  "#e7c6ff", // lilac
] as const;

interface SessionState {
  name: string;
  color: string;
  setName: (name: string) => void;
  setColor: (color: string) => void;
}

/** What the user picked on the landing page, remembered across visits. */
export const useSession = create<SessionState>()(
  persist(
    (set) => ({
      name: "",
      color: DOODLE_COLORS[0],
      setName: (name) => set({ name }),
      setColor: (color) => set({ color }),
    }),
    { name: "echoroom.session" },
  ),
);

/**
 * True once the persisted store has been restored from localStorage.
 * Always false during server rendering, so server and client HTML match.
 */
export function useSessionHydrated(): boolean {
  return useSyncExternalStore(
    (cb) => useSession.persist.onFinishHydration(cb),
    () => useSession.persist.hasHydrated(),
    () => false,
  );
}
