import { Fredoka } from "next/font/google";

// Rounded, friendly font used by both the HTML UI and the Phaser canvas.
export const fredoka = Fredoka({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-fredoka",
});
