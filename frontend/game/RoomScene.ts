import * as Phaser from "phaser";
import { MAX_SPEED } from "@/lib/config";
import type { Dir, Player } from "@/lib/protocol";
import type { RoomSocket } from "@/lib/roomSocket";
import { Doodle } from "./Doodle";

const SEND_INTERVAL_MS = 66; // ~15 Hz; server broadcasts at 10 Hz
const REMOTE_SMOOTHING = 12; // higher = snappier remote doodles
const ARRIVE_EPS = 4; // px; tap-to-move stops this close to the target

export interface RoomSceneOptions {
  socket: RoomSocket;
  font: string;
  onPlayerCount: (n: number) => void;
}

/**
 * Movement model:
 *  - Your doodle moves locally every frame (no waiting for the server).
 *  - Position is sent ~15×/s while it changes, plus once when you stop.
 *  - Others glide toward their latest server position (exponential smoothing),
 *    which hides the 100 ms gaps between server ticks.
 */
export class RoomScene extends Phaser.Scene {
  private opts!: RoomSceneOptions;
  private doodles = new Map<string, Doodle>();
  private selfId: string | null = null;
  private world = { w: 1280, h: 800 };
  private floor?: Phaser.GameObjects.Graphics;

  private keys!: Record<"up" | "down" | "left" | "right" | "w" | "a" | "s" | "d", Phaser.Input.Keyboard.Key>;
  private tapTarget: { x: number; y: number } | null = null;
  private lastSent = { x: NaN, y: NaN, moving: false, dir: "right" as Dir, at: 0 };
  private unsubs: Array<() => void> = [];

  constructor() {
    super("room");
  }

  init(opts: RoomSceneOptions) {
    this.opts = opts;
  }

  create() {
    this.cameras.main.setBackgroundColor("#f6efe6");
    this.drawFloor();

    const kb = this.input.keyboard!;
    const K = Phaser.Input.Keyboard.KeyCodes;
    this.keys = kb.addKeys({
      up: K.UP, down: K.DOWN, left: K.LEFT, right: K.RIGHT, w: K.W, a: K.A, s: K.S, d: K.D,
    }, false) as typeof this.keys;
    // Don't swallow keys when typing in an HTML input (needed once chat exists).
    kb.disableGlobalCapture();

    this.input.on("pointerdown", (p: Phaser.Input.Pointer) => {
      this.tapTarget = { x: p.worldX, y: p.worldY };
    });

    const s = this.opts.socket;
    this.unsubs.push(
      s.on("welcome", (m) => {
        // Also fires after every reconnect: rebuild everything from scratch.
        this.doodles.forEach((d) => d.destroy());
        this.doodles.clear();
        this.selfId = m.selfId;
        this.world = m.world;
        this.drawFloor();
        m.players.forEach((p) => this.addDoodle(p));
        const me = this.doodles.get(m.selfId);
        if (me) this.cameras.main.startFollow(me, true, 0.15, 0.15);
        this.reportCount();
      }),
      s.on("joined", (p) => {
        this.addDoodle(p);
        this.reportCount();
      }),
      s.on("left", ({ id }) => {
        this.doodles.get(id)?.destroy();
        this.doodles.delete(id);
        this.reportCount();
      }),
      s.on("state", ({ players }) => {
        for (const p of players) {
          if (p.id === this.selfId) continue; // we already know where we are
          const d = this.doodles.get(p.id);
          if (!d) continue;
          d.targetX = p.x;
          d.targetY = p.y;
          d.setFacing(p.dir);
          d.setMoving(p.moving);
        }
      }),
      s.on("correct", ({ x, y }) => {
        const me = this.self();
        if (!me) return;
        me.setPosition(x, y);
        this.tapTarget = null;
      }),
    );

    this.events.once(Phaser.Scenes.Events.SHUTDOWN, () => this.unsubs.forEach((u) => u()));

    // Subscribe first, connect second, so the welcome frame is never missed.
    s.connect();
  }

  update(_time: number, deltaMs: number) {
    const dt = Math.min(deltaMs, 100) / 1000;
    this.moveSelf(dt);

    for (const d of this.doodles.values()) {
      if (d.playerId !== this.selfId) {
        const k = 1 - Math.exp(-REMOTE_SMOOTHING * dt);
        d.x += (d.targetX - d.x) * k;
        d.y += (d.targetY - d.y) * k;
      }
      d.animate(dt);
    }
  }

  private self() {
    return this.selfId ? this.doodles.get(this.selfId) : undefined;
  }

  private moveSelf(dt: number) {
    const me = this.self();
    if (!me) return;

    const k = this.keys;
    let vx = (k.right.isDown || k.d.isDown ? 1 : 0) - (k.left.isDown || k.a.isDown ? 1 : 0);
    let vy = (k.down.isDown || k.s.isDown ? 1 : 0) - (k.up.isDown || k.w.isDown ? 1 : 0);

    if (vx !== 0 || vy !== 0) {
      this.tapTarget = null; // keyboard overrides tap-to-move
    } else if (this.tapTarget) {
      const dx = this.tapTarget.x - me.x;
      const dy = this.tapTarget.y - me.y;
      if (Math.hypot(dx, dy) < ARRIVE_EPS) this.tapTarget = null;
      else [vx, vy] = [dx, dy];
    }

    const len = Math.hypot(vx, vy);
    const moving = len > 0;
    if (moving) {
      // Normalise so diagonals aren't faster, and never overshoot a tap target.
      let step = MAX_SPEED * dt;
      if (this.tapTarget) step = Math.min(step, len);
      me.x = Phaser.Math.Clamp(me.x + (vx / len) * step, 0, this.world.w);
      me.y = Phaser.Math.Clamp(me.y + (vy / len) * step, 0, this.world.h);
      if (vx !== 0) me.setFacing(vx > 0 ? "right" : "left");
    }
    me.setMoving(moving);
    this.maybeSend(me, moving);
  }

  private maybeSend(me: Doodle, moving: boolean) {
    const now = this.time.now;
    const dir = me.facing;
    const changed = me.x !== this.lastSent.x || me.y !== this.lastSent.y;
    const stopped = !moving && this.lastSent.moving;

    if ((changed && now - this.lastSent.at >= SEND_INTERVAL_MS) || stopped) {
      const x = Math.round(me.x * 10) / 10;
      const y = Math.round(me.y * 10) / 10;
      this.opts.socket.send("move", { x, y, dir, moving });
      this.lastSent = { x: me.x, y: me.y, moving, dir, at: now };
    }
  }

  private addDoodle(p: Player) {
    if (this.doodles.has(p.id)) return;
    const d = new Doodle(this, {
      id: p.id,
      x: p.x,
      y: p.y,
      name: p.name,
      color: p.avatar.color,
      isSelf: p.id === this.selfId,
      font: this.opts.font,
    });
    d.setFacing(p.dir);
    d.setMoving(p.moving);
    this.doodles.set(p.id, d);
  }

  private reportCount() {
    this.opts.onPlayerCount(this.doodles.size);
  }

  /** Placeholder room until Phase 4 brings a real tilemap. */
  private drawFloor() {
    const { w, h } = this.world;
    this.floor?.destroy();
    const g = (this.floor = this.add.graphics().setDepth(-10_000));
    const tile = 64;
    for (let y = 0; y < h; y += tile) {
      for (let x = 0; x < w; x += tile) {
        g.fillStyle(((x + y) / tile) % 2 === 0 ? 0xfdf6ec : 0xf7ead9, 1);
        g.fillRect(x, y, tile, tile);
      }
    }
    // rug
    g.fillStyle(0xffd6e0, 1).fillRoundedRect(w / 2 - 220, h / 2 - 130, 440, 260, 40);
    g.lineStyle(4, 0xffb3c6, 1).strokeRoundedRect(w / 2 - 220, h / 2 - 130, 440, 260, 40);
    // walls
    g.lineStyle(10, 0x2b2b3a, 1).strokeRoundedRect(0, 0, w, h, 24);
    this.cameras.main.setBounds(-40, -40, w + 80, h + 80);
  }
}
