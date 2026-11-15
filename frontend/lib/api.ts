import { getAccessToken } from "./auth";
import { API_URL } from "./config";
import type { Profile } from "./protocol";

export interface Ticket {
  ticket: string;
  roomId: string;
  expiresIn: number;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

/** Trades the JWT (sent in a header) for a one-time WebSocket ticket. */
export async function requestTicket(profile: Profile, signal?: AbortSignal): Promise<Ticket> {
  const token = await getAccessToken();
  const res = await fetch(`${API_URL}/api/v1/ws/ticket`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ profile }),
    signal,
  });
  if (!res.ok) {
    throw new ApiError(res.status, (await res.text()).trim() || res.statusText);
  }
  return res.json();
}
