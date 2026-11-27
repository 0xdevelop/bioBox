export type FlywheelLane = {
  name: string;
  lines: string[];
};

const TURN = Math.PI * 2;
const FONT = "ui-monospace, SFMono-Regular, Menlo, monospace";

type DialTone = {
  ink: string;
  dim: string;
  arc: string;
};

const tones: DialTone[] = [
  { ink: "#f2d7a6", dim: "#8d7348", arc: "#e0a15a" },
  { ink: "#d5e7f2", dim: "#6d8ea3", arc: "#7eb6d6" },
];

function laneTone(index: number): DialTone {
  return tones[index] ?? tones[0];
}

function activeLine(lines: string[]): boolean {
  return lines.some((line) => line.includes("running") || line.includes("queued") || line.includes("submitting"));
}

function drawRing(
  context: CanvasRenderingContext2D,
  cx: number,
  cy: number,
  radius: number,
  spin: number,
  tone: DialTone,
  lines: string[],
  tickCount: number,
): void {
  context.save();
  context.translate(cx, cy);
  context.rotate(spin);
  context.strokeStyle = tone.arc;
  context.globalAlpha = 0.85;
  context.lineWidth = 2;
  context.beginPath();
  context.arc(0, 0, radius, 0, TURN);
  context.stroke();

  context.lineWidth = 1;
  for (let index = 0; index < tickCount; index += 1) {
    const angle = (index / tickCount) * TURN;
    const long = index % 5 === 0;
    const inner = radius - (long ? 14 : 7);
    context.globalAlpha = long ? 0.95 : 0.45;
    context.beginPath();
    context.moveTo(Math.cos(angle) * inner, Math.sin(angle) * inner);
    context.lineTo(Math.cos(angle) * (radius - 2), Math.sin(angle) * (radius - 2));
    context.stroke();
  }

  const sample = lines.length > 0 ? lines : ["idle"];
  context.font = `500 12px ${FONT}`;
  context.textAlign = "left";
  context.textBaseline = "middle";
  context.fillStyle = tone.ink;
  sample.slice(0, 6).forEach((line, index) => {
    const angle = (index / 6) * TURN;
    context.save();
    context.rotate(angle);
    context.globalAlpha = 0.9;
    const shown = line.length > 22 ? `${line.slice(0, 20)}..` : line;
    context.fillText(shown, radius * 0.42, 0);
    context.restore();
  });
  context.restore();
}

function drawCore(
  context: CanvasRenderingContext2D,
  cx: number,
  cy: number,
  lanes: FlywheelLane[],
): void {
  context.beginPath();
  context.arc(cx, cy, 54, 0, TURN);
  context.fillStyle = "#1a2030";
  context.fill();
  context.strokeStyle = "#3a4458";
  context.lineWidth = 1;
  context.stroke();
  context.font = `600 11px ${FONT}`;
  context.textAlign = "center";
  context.textBaseline = "middle";
  lanes.forEach((lane, index) => {
    const tone = laneTone(index);
    context.fillStyle = activeLine(lane.lines) ? tone.arc : tone.dim;
    context.fillText(lane.name, cx, cy + (index === 0 ? -10 : 12));
  });
}

export function mountFlywheel(canvas: HTMLCanvasElement, readLanes: () => FlywheelLane[]): () => void {
  const stage = canvas.parentElement;
  if (stage === null) {
    throw new Error("dial stage is missing");
  }
  let frame = 0;
  let last = 0;
  let spinA = 0;
  let spinB = 0;

  const paint = (now: number) => {
    frame = requestAnimationFrame(paint);
    const context = canvas.getContext("2d");
    if (context === null) {
      return;
    }
    const bounds = stage.getBoundingClientRect();
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const width = Math.max(1, Math.floor(bounds.width));
    const height = Math.max(1, Math.floor(bounds.height));
    const pixelWidth = Math.round(width * dpr);
    const pixelHeight = Math.round(height * dpr);
    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
      canvas.width = pixelWidth;
      canvas.height = pixelHeight;
    }
    if (last === 0) {
      last = now;
    }
    const dt = Math.min(0.05, (now - last) / 1000);
    last = now;
    const lanes = readLanes();
    const outerSpeed = activeLine(lanes[0]?.lines ?? []) ? 0.55 : 0.22;
    const innerSpeed = activeLine(lanes[1]?.lines ?? []) ? -0.7 : -0.28;
    spinA += dt * outerSpeed;
    spinB += dt * innerSpeed;

    context.setTransform(dpr, 0, 0, dpr, 0, 0);
    context.clearRect(0, 0, width, height);
    context.fillStyle = "#12151c";
    context.fillRect(0, 0, width, height);

    const cx = width * 0.5;
    const cy = height * 0.5;
    const outer = Math.max(96, Math.min(width, height) * 0.38);
    const inner = outer * 0.62;

    context.beginPath();
    context.arc(cx, cy, outer + 18, 0, TURN);
    context.strokeStyle = "#2a3344";
    context.lineWidth = 1;
    context.stroke();

    drawRing(context, cx, cy, outer, spinA, laneTone(0), lanes[0]?.lines ?? ["idle"], 48);
    drawRing(context, cx, cy, inner, spinB, laneTone(1), lanes[1]?.lines ?? ["idle"], 32);
    drawCore(context, cx, cy, lanes);
  };

  frame = requestAnimationFrame(paint);
  return () => {
    cancelAnimationFrame(frame);
  };
}
