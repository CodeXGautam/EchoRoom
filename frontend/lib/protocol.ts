// Mirrors backend/internal/room/protocol.go. Keep the two in sync.

export type Dir = "left" | "right";

export interface Avatar {
  color: string;
}

export interface Profile {
  name: string;
  avatar: Avatar;
}

export interface Player {
  id: string;
  name: string;
  avatar: Avatar;
  x: number;
  y: number;
  dir: Dir;
  moving: boolean;
}

export type PlayerState = Pick<Player, "id" | "x" | "y" | "dir" | "moving">;

export interface ServerMessages {
  welcome: { selfId: string; world: { w: number; h: number }; players: Player[] };
  joined: Player;
  left: { id: string };
  state: { players: PlayerState[] };
  correct: { x: number; y: number };
}

export interface ClientMessages {
  move: { x: number; y: number; dir: Dir; moving: boolean };
}

// Application close codes (backend room.Close*).
export const CLOSE_REPLACED = 4001;
export const CLOSE_TOO_SLOW = 4002;
export const CLOSE_FLOODING = 4003;
