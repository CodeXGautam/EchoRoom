import type { Metadata } from "next";
import { fredoka } from "./fonts";
import "./globals.css";

export const metadata: Metadata = {
  title: "EchoRoom",
  description: "A cosy room where little doodles wander around and chat.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${fredoka.variable} h-full antialiased`}>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
