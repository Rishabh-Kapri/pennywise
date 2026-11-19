import { useId } from 'react';
import styles from './PennywiseMark.module.css';

interface PennywiseMarkProps {
  /** Play the loop: emails fall into the water and become its lines, then the sun rises. */
  animated?: boolean;
  className?: string;
  /** Accessible name; leave empty when text beside the mark already names it. */
  title?: string;
}

const LINES = [
  { x: 160, y: 300, width: 192 },
  { x: 188, y: 326, width: 136 },
  { x: 216, y: 352, width: 80 },
  { x: 240, y: 378, width: 32 }
];

/**
 * The Pennywise icon (docs/logo/pennywise-icon.svg). Still, it is the loop's
 * last frame, the same picture as public/logo-mark.svg and the app icons.
 * The four envelopes and lines play staggered by 0.7 s in an 8 s loop
 * (docs/logo/pennywise-icon-animated.svg); android's PennywiseMark plays the
 * same keyframes, so change both together.
 */
export function PennywiseMark({ animated = false, className, title }: PennywiseMarkProps) {
  // clip ids must be unique per instance on the page
  const id = useId();
  const tile = `${id}-tile`;
  const sky = `${id}-sky`;
  const staggered = (i: number) => styles[`d${i}`] ?? '';

  return (
    <svg
      className={`${animated ? styles.animated : ''} ${className ?? ''}`}
      viewBox="0 0 512 512"
      role={title ? 'img' : undefined}
      aria-label={title || undefined}
      aria-hidden={title ? undefined : true}>
      <defs>
        <clipPath id={tile}>
          <rect width="512" height="512" rx="116" />
        </clipPath>
        <clipPath id={sky}>
          <rect width="512" height="282" />
        </clipPath>
      </defs>
      <g className={styles.tile} clipPath={`url(#${tile})`}>
        <rect width="512" height="290" fill="#3176AA" />
        <rect y="282" width="512" height="230" fill="#0D1A26" />
        <g clipPath={`url(#${sky})`}>
          <path className={styles.sun} d="M144 282A112 112 0 0 1 368 282Z" fill="#FFFFFF" />
          {animated &&
            LINES.map((_, i) => (
              <g key={i} className={`${styles.env} ${staggered(i)}`}>
                <rect x="226" y="240" width="60" height="42" rx="8" fill="#FFFFFF" />
                <path
                  d="M235 251L256 266L277 251"
                  fill="none"
                  stroke="#3176AA"
                  strokeWidth="5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </g>
            ))}
        </g>
        <g fill="#ADD2EB">
          {LINES.map((line, i) => (
            <rect key={i} className={`${styles.line} ${staggered(i)}`} x={line.x} y={line.y} width={line.width} height="12" rx="6" />
          ))}
        </g>
      </g>
    </svg>
  );
}

export default PennywiseMark;
