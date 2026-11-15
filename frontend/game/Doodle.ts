import * as Phaser from "phaser";
import type { Dir } from "@/lib/protocol";

const INK = 0x2b2b3a;
const BLUSH = 0xff8fab;

/**
 * A blob doodle drawn entirely with vector shapes — no sprite sheet needed.
 * Walking = bobbing body + alternating feet; facing = eyes shift sideways.
 *
 * Origin (0,0) is the doodle's feet, so y-sorting by container.y works.
 */
export class Doodle extends Phaser.GameObjects.Container {
  readonly playerId: string;
  private bodyG: Phaser.GameObjects.Graphics;
  private faceG: Phaser.GameObjects.Graphics;
  private footL: Phaser.GameObjects.Graphics;
  private footR: Phaser.GameObjects.Graphics;
  private label: Phaser.GameObjects.Text;
  private color: number;
  private dir: Dir = "right";
  private moving = false;
  private phase = Math.random() * Math.PI * 2;

  /** Where the network says this doodle is; remote doodles glide toward it. */
  targetX: number;
  targetY: number;

  constructor(
    scene: Phaser.Scene,
    opts: { id: string; x: number; y: number; name: string; color: string; isSelf: boolean; font: string },
  ) {
    super(scene, opts.x, opts.y);
    this.playerId = opts.id;
    this.targetX = opts.x;
    this.targetY = opts.y;
    this.color = Phaser.Display.Color.HexStringToColor(opts.color).color;

    const shadow = scene.add.graphics();
    shadow.fillStyle(0x000000, 0.12).fillEllipse(0, 0, 40, 10);

    this.footL = this.makeFoot(scene, -9);
    this.footR = this.makeFoot(scene, 9);

    this.bodyG = scene.add.graphics();
    this.drawBody();

    this.faceG = scene.add.graphics();
    this.drawFace();

    this.label = scene.add
      .text(0, 12, opts.name, {
        fontFamily: opts.font,
        fontSize: "14px",
        color: "#2b2b3a",
        backgroundColor: opts.isSelf ? "#fff4b8" : "#ffffffcc",
        padding: { x: 6, y: 2 },
      })
      .setOrigin(0.5, 0);

    this.add([shadow, this.footL, this.footR, this.bodyG, this.faceG, this.label]);
    scene.add.existing(this);
  }

  private makeFoot(scene: Phaser.Scene, x: number) {
    const g = scene.add.graphics({ x, y: -3 });
    g.fillStyle(this.color, 1).fillEllipse(0, 0, 12, 8);
    g.lineStyle(2.5, INK, 1).strokeEllipse(0, 0, 12, 8);
    return g;
  }

  private drawBody() {
    const g = this.bodyG.clear();
    g.fillStyle(this.color, 1).fillEllipse(0, -24, 46, 42);
    g.lineStyle(3, INK, 1).strokeEllipse(0, -24, 46, 42);
    // tiny highlight so it looks soft
    g.fillStyle(0xffffff, 0.55).fillEllipse(-10, -36, 10, 6);
  }

  private drawFace() {
    const s = this.dir === "right" ? 1 : -1;
    const g = this.faceG.clear();
    g.fillStyle(INK, 1);
    g.fillEllipse(s * 4 - 7, -27, 5, 7);
    g.fillEllipse(s * 4 + 7, -27, 5, 7);
    g.fillStyle(BLUSH, 0.6);
    g.fillEllipse(s * 4 - 13, -19, 8, 4);
    g.fillEllipse(s * 4 + 13, -19, 8, 4);
    g.lineStyle(2, INK, 1);
    g.beginPath();
    g.arc(s * 4, -22, 3, 0.15 * Math.PI, 0.85 * Math.PI);
    g.strokePath();
  }

  get facing(): Dir {
    return this.dir;
  }

  setFacing(dir: Dir) {
    if (dir !== this.dir) {
      this.dir = dir;
      this.drawFace();
    }
  }

  setMoving(moving: boolean) {
    this.moving = moving;
  }

  /** Per-frame animation. dt in seconds. */
  animate(dt: number) {
    if (this.moving) {
      this.phase += dt * 14;
      const bob = Math.abs(Math.sin(this.phase)) * 4;
      this.bodyG.y = this.faceG.y = -bob;
      this.footL.y = -3 - Math.max(0, Math.sin(this.phase)) * 4;
      this.footR.y = -3 - Math.max(0, -Math.sin(this.phase)) * 4;
    } else {
      // gentle idle "breathing"
      this.phase += dt * 2.5;
      this.bodyG.y = this.faceG.y = Math.sin(this.phase) * 0.8;
      this.footL.y = this.footR.y = -3;
    }
    this.setDepth(this.y); // lower on screen = drawn in front
  }
}
