/** SVG twin of game/Doodle.ts, for the landing page. Keep shapes in sync. */
export default function DoodlePreview({ color, size = 140 }: { color: string; size?: number }) {
  const ink = "#2b2b3a";
  return (
    <svg width={size} height={size} viewBox="-40 -60 80 70" aria-hidden className="doodle-bob">
      <ellipse cx="0" cy="0" rx="20" ry="5" fill="#000" opacity="0.12" />
      <ellipse cx="-9" cy="-3" rx="6" ry="4" fill={color} stroke={ink} strokeWidth="2.5" />
      <ellipse cx="9" cy="-3" rx="6" ry="4" fill={color} stroke={ink} strokeWidth="2.5" />
      <ellipse cx="0" cy="-24" rx="23" ry="21" fill={color} stroke={ink} strokeWidth="3" />
      <ellipse cx="-10" cy="-36" rx="5" ry="3" fill="#fff" opacity="0.55" />
      <ellipse cx="-3" cy="-27" rx="2.5" ry="3.5" fill={ink} />
      <ellipse cx="11" cy="-27" rx="2.5" ry="3.5" fill={ink} />
      <ellipse cx="-9" cy="-19" rx="4" ry="2" fill="#ff8fab" opacity="0.6" />
      <ellipse cx="17" cy="-19" rx="4" ry="2" fill="#ff8fab" opacity="0.6" />
      <path d="M 1.4 -19.6 A 3 3 0 0 0 6.6 -19.6" fill="none" stroke={ink} strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}
