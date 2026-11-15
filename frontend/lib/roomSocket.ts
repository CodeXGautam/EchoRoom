import { ApiError, requestTicket } from "./api";
import { SignInRequiredError } from "./auth";
import { WS_URL } from "./config";
import {
  CLOSE_REPLACED,
  type ClientMessages,
  type Profile,
  type ServerMessages,
} from "./protocol";

export type SocketStatus =
  | "connecting" // first attempt (may be slow: free Render instance waking up)
  | "open"
  | "reconnecting"
  | "replaced" // same user opened another tab; we stop here
  | "failed" // permanent error (bad profile, auth rejected)
  | "closed"; // we closed it

type Handler<K extends keyof ServerMessages> = (data: ServerMessages[K]) => void;

/**
 * One WebSocket to one room, with automatic reconnect.
 *
 * Every (re)connect fetches a fresh one-time ticket. After a reconnect the
 * server sends a new `welcome`, so listeners must rebuild state from it.
 */
export class RoomSocket {
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<(d: unknown) => void>>();
  private statusListeners = new Set<(s: SocketStatus, detail?: string) => void>();
  private attempt = 0;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;
  private abort = new AbortController();
  private stopped = false;

  status: SocketStatus = "connecting";

  constructor(private profile: Profile) {}

  on<K extends keyof ServerMessages>(type: K, fn: Handler<K>): () => void {
    let set = this.handlers.get(type);
    if (!set) this.handlers.set(type, (set = new Set()));
    set.add(fn as (d: unknown) => void);
    return () => set.delete(fn as (d: unknown) => void);
  }

  onStatus(fn: (s: SocketStatus, detail?: string) => void): () => void {
    this.statusListeners.add(fn);
    fn(this.status);
    return () => this.statusListeners.delete(fn);
  }

  send<K extends keyof ClientMessages>(t: K, d: ClientMessages[K]) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ t, d }));
    }
  }

  connect() {
    void this.open();
  }

  close() {
    this.stopped = true;
    this.abort.abort();
    if (this.retryTimer) clearTimeout(this.retryTimer);
    this.ws?.close(1000, "bye");
    this.setStatus("closed");
  }

  private setStatus(s: SocketStatus, detail?: string) {
    this.status = s;
    this.statusListeners.forEach((fn) => fn(s, detail));
  }

  private async open() {
    if (this.stopped) return;
    this.setStatus(this.attempt === 0 ? "connecting" : "reconnecting");

    let ticket: string;
    try {
      ticket = (await requestTicket(this.profile, this.abort.signal)).ticket;
    } catch (err) {
      if (this.stopped) return;
      // 400/401 won't fix themselves by retrying.
      if (err instanceof ApiError && (err.status === 400 || err.status === 401)) {
        this.setStatus("failed", err.message);
        return;
      }
      if (err instanceof SignInRequiredError) {
        this.setStatus("failed", err.message);
        return;
      }
      this.scheduleRetry();
      return;
    }
    if (this.stopped) return;

    const ws = new WebSocket(`${WS_URL}/ws?ticket=${encodeURIComponent(ticket)}`);
    this.ws = ws;

    ws.onopen = () => {
      this.attempt = 0;
      this.setStatus("open");
    };
    ws.onmessage = (ev) => {
      let msg: { t: string; d: unknown };
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      this.handlers.get(msg.t)?.forEach((fn) => fn(msg.d));
    };
    ws.onclose = (ev) => {
      if (this.ws !== ws || this.stopped) return;
      this.ws = null;
      if (ev.code === CLOSE_REPLACED) {
        this.setStatus("replaced");
        return;
      }
      this.scheduleRetry();
    };
  }

  /** Exponential backoff with jitter: ~0.5s, 1s, 2s, 4s … capped at 10s. */
  private scheduleRetry() {
    this.attempt++;
    const base = Math.min(10_000, 500 * 2 ** (this.attempt - 1));
    const delay = base / 2 + Math.random() * (base / 2);
    this.setStatus("reconnecting");
    this.retryTimer = setTimeout(() => void this.open(), delay);
  }
}
