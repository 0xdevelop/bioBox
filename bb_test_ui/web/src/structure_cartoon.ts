import * as THREE from "three";

// Cartoon (ribbon) support for the real-coordinate structure stage.
// Every vertex is derived from artifact coordinates: the ribbon centre line is a
// Catmull-Rom spline through the real Cα positions (it passes through each Cα),
// and its width direction comes from the real Cα→O vector of each residue.
// Secondary structure comes from the file's own records when present
// (_struct_conf / _struct_sheet_range, or PDB HELIX / SHEET); otherwise it is
// INFERRED with a simplified DSSP (backbone N–H···O=C electrostatic energy).

export interface CartoonAtom {
  x: number; y: number; z: number; element: string; name: string;
  chain: string; residue: string; seq: number | null; model: string;
  segment?: number; insertion?: string;
}
export type SecondaryType = "H" | "E" | "C";
export interface SecondaryRange { chain: string; start: number; end: number; type: "H" | "E" }
export interface SecondaryRecords { ranges: SecondaryRange[]; source: "cif" | "pdb" | "none" }
export type Vec3 = [number, number, number];
export interface CartoonResidue {
  model: string; chain: string; segment: number; seq: number; insertion: string; name: string;
  ca: Vec3; n: Vec3 | null; c: Vec3 | null; o: Vec3 | null; caIndex: number;
}
export interface CartoonBreak { chain: string; from: number; to: number; reason: string }
export type SecondarySource = "records-cif" | "records-pdb" | "inferred-dssp" | "inferred-none";
export interface SecondaryAssignment { ss: SecondaryType[]; source: SecondarySource; note: string; hbonds: number }

export const CA_MIN = 2.5;
export const CA_MAX = 4.5;
const AMINO = new Set("ALA ARG ASN ASP CYS GLN GLU GLY HIS ILE LEU LYS MET PHE PRO SER THR TRP TYR VAL MSE SEC PYL".split(" "));

function cifTokens(line: string): string[] {
  return (line.match(/'(?:[^']*)'|"(?:[^"]*)"|\S+/g) || []).map((v) => v.replace(/^(['"])(.*)\1$/, "$2"));
}
const cifClean = (v: string | undefined) => (v === undefined || v === "." || v === "?" ? "" : v);

// Reads one mmCIF category in either loop_ or single key-value form.
export function cifCategory(text: string, category: string): Record<string, string>[] {
  const lines = String(text).split(/\r?\n/);
  const prefix = "_" + category + ".";
  const rows: Record<string, string>[] = [];
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (t === "loop_") {
      let j = i + 1;
      const headers: string[] = [];
      while (j < lines.length && lines[j].trim().startsWith("_")) { headers.push(lines[j].trim().split(/\s+/)[0]); j++; }
      if (!headers.length || !headers[0].startsWith(prefix)) { i = j - 1; continue; }
      const buffer: string[] = [];
      for (; j < lines.length; j++) {
        const raw = lines[j];
        const v = raw.trim();
        if (raw.startsWith(";")) {
          const parts = [raw.slice(1)];
          j++;
          while (j < lines.length && !lines[j].startsWith(";")) { parts.push(lines[j]); j++; }
          buffer.push(parts.join("\n").trim());
        } else {
          if (!v || v.startsWith("#") || v === "loop_" || v.startsWith("_") || v.startsWith("data_")) break;
          buffer.push(...cifTokens(v));
        }
        while (buffer.length >= headers.length) {
          const values = buffer.splice(0, headers.length);
          rows.push(Object.fromEntries(headers.map((h, k) => [h.slice(prefix.length), cifClean(values[k])])));
        }
      }
      i = j - 1;
    } else if (t.startsWith(prefix)) {
      const row: Record<string, string> = {};
      let j = i;
      while (j < lines.length && lines[j].trim().startsWith(prefix)) {
        const tokens = cifTokens(lines[j].trim());
        let value = tokens[1];
        if (value === undefined) {
          const next = lines[j + 1] ?? "";
          if (next.startsWith(";")) {
            const parts = [next.slice(1)];
            j += 2;
            while (j < lines.length && !lines[j].startsWith(";")) { parts.push(lines[j]); j++; }
            value = parts.join("\n").trim();
          } else { value = cifTokens(next.trim())[0]; j++; }
        }
        row[tokens[0].slice(prefix.length)] = cifClean(value);
        j++;
      }
      rows.push(row);
      i = j - 1;
    }
  }
  return rows;
}

// Secondary-structure records exactly as written in the file. Nothing is inferred here.
export function parseSecondaryRecords(text: string): SecondaryRecords {
  const src = String(text || "");
  const ranges: SecondaryRange[] = [];
  const looksCIF = /^data_/m.test(src) || src.includes("_atom_site.");
  if (looksCIF) {
    const add = (row: Record<string, string>, type: "H" | "E") => {
      const chain = row.beg_label_asym_id || row.beg_auth_asym_id || "";
      const endChain = row.end_label_asym_id || row.end_auth_asym_id || chain;
      const start = Number(row.beg_label_seq_id || row.beg_auth_seq_id);
      const end = Number(row.end_label_seq_id || row.end_auth_seq_id);
      if (chain !== endChain || !Number.isFinite(start) || !Number.isFinite(end)) return;
      ranges.push({ chain, start: Math.min(start, end), end: Math.max(start, end), type });
    };
    for (const row of cifCategory(src, "struct_conf")) if (/^HELX/i.test(row.conf_type_id || "")) add(row, "H");
    for (const row of cifCategory(src, "struct_sheet_range")) add(row, "E");
    return { ranges, source: ranges.length ? "cif" : "none" };
  }
  for (const line of src.split(/\r?\n/)) {
    if (line.startsWith("HELIX ")) {
      const chain = line.slice(19, 20).trim(), endChain = line.slice(31, 32).trim();
      const start = parseInt(line.slice(21, 25), 10), end = parseInt(line.slice(33, 37), 10);
      if (chain === endChain && Number.isFinite(start) && Number.isFinite(end)) ranges.push({ chain, start, end, type: "H" });
    } else if (line.startsWith("SHEET ")) {
      const chain = line.slice(21, 22).trim(), endChain = line.slice(32, 33).trim();
      const start = parseInt(line.slice(22, 26), 10), end = parseInt(line.slice(33, 37), 10);
      if (chain === endChain && Number.isFinite(start) && Number.isFinite(end)) ranges.push({ chain, start, end, type: "E" });
    }
  }
  return { ranges, source: ranges.length ? "pdb" : "none" };
}

// Groups atoms of one already-filtered MODEL into amino-acid residues with a real Cα.
export function collectResidues(atoms: CartoonAtom[]): CartoonResidue[] {
  const byKey = new Map<string, CartoonResidue & { keep: boolean }>();
  const out: (CartoonResidue & { keep: boolean })[] = [];
  atoms.forEach((a, index) => {
    if (a.seq == null || !Number.isFinite(a.seq)) return;
    const key = [a.model, a.chain, a.segment ?? 0, a.seq, a.insertion ?? ""].join("|");
    let r = byKey.get(key);
    if (!r) {
      r = { model: a.model, chain: a.chain, segment: a.segment ?? 0, seq: a.seq, insertion: a.insertion ?? "", name: a.residue,
        ca: [NaN, NaN, NaN], n: null, c: null, o: null, caIndex: -1, keep: false };
      byKey.set(key, r); out.push(r);
    }
    const p: Vec3 = [a.x, a.y, a.z];
    if (a.name === "CA" && a.element === "C" && r.caIndex < 0) { r.ca = p; r.caIndex = index; }
    else if (a.name === "N" && a.element === "N" && !r.n) r.n = p;
    else if (a.name === "C" && a.element === "C" && !r.c) r.c = p;
    else if (a.name === "O" && a.element === "O" && !r.o) r.o = p;
  });
  return out.filter((r) => r.caIndex >= 0 && (AMINO.has(r.name) || (!!r.n && !!r.c))).map(({ keep: _keep, ...r }) => r);
}

const dist = (a: Vec3, b: Vec3) => Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);

// Splits residues into ribbon segments. A ribbon never crosses a chain/MODEL/TER
// boundary, a residue-number gap, an insertion code, or a Cα–Cα distance outside 2.5–4.5 Å.
export function splitSegments(res: CartoonResidue[]): { segments: number[][]; breaks: CartoonBreak[] } {
  const segments: number[][] = [];
  const breaks: CartoonBreak[] = [];
  res.forEach((r, i) => {
    const p = i ? res[i - 1] : null;
    let reason = "";
    if (!p || p.model !== r.model || p.chain !== r.chain) reason = "new-chain";
    else if (p.segment !== r.segment) reason = "TER 分段";
    else if (p.insertion || r.insertion) reason = "插入码";
    else if (r.seq !== p.seq + 1) reason = "残基序号不连续";
    else {
      const d = dist(p.ca, r.ca);
      if (d < CA_MIN || d > CA_MAX) reason = "Cα 距离 " + d.toFixed(2) + " Å 超出 2.5–4.5 Å";
    }
    if (reason) {
      if (reason !== "new-chain") breaks.push({ chain: r.chain, from: p!.seq, to: r.seq, reason });
      segments.push([i]);
    } else segments[segments.length - 1].push(i);
  });
  return { segments, breaks };
}

// Simplified DSSP (Kabsch & Sander 1983): H-bond if E < -0.5 kcal/mol with
// E = 0.084·332·(1/rON + 1/rCH − 1/rOH − 1/rCN); amide H placed 1.0 Å from N
// opposite the previous C=O. Only α-helix (two consecutive 4-turns) and
// β-ladders (two consecutive bridges) are assigned; 3₁₀/π/turn/bulge are not.
export function inferSecondary(res: CartoonResidue[], segments: number[][]): { ss: SecondaryType[]; hbonds: number; usable: number } {
  const n = res.length;
  const ss: SecondaryType[] = new Array(n).fill("C");
  const segOf = new Int32Array(n).fill(-1);
  segments.forEach((seg, s) => seg.forEach((i) => { segOf[i] = s; }));
  const near = (i: number, d: number) => { const j = i + d; return j >= 0 && j < n && segOf[j] === segOf[i] ? j : -1; };
  const hPos: (Vec3 | null)[] = res.map((r, j) => {
    const p = near(j, -1);
    if (p < 0 || !r.n || r.name === "PRO" || !res[p].c || !res[p].o) return null;
    const c = res[p].c!, o = res[p].o!;
    const v: Vec3 = [c[0] - o[0], c[1] - o[1], c[2] - o[2]];
    const len = Math.hypot(...v) || 1;
    return [r.n[0] + v[0] / len, r.n[1] + v[1] / len, r.n[2] + v[2] / len];
  });
  const usable = res.filter((r) => r.n && r.c && r.o).length;
  const hb = new Set<number>();
  const cell = 9;
  const grid = new Map<string, number[]>();
  const cellKey = (p: Vec3) => [Math.floor(p[0] / cell), Math.floor(p[1] / cell), Math.floor(p[2] / cell)];
  res.forEach((r, i) => { const k = cellKey(r.ca).join(","); (grid.get(k) || grid.set(k, []).get(k)!).push(i); });
  const q = 0.084 * 332;
  let hbonds = 0;
  for (let i = 0; i < n; i++) {
    const acc = res[i];
    if (!acc.c || !acc.o) continue;
    const [cx, cy, cz] = cellKey(acc.ca);
    for (let dx = -1; dx <= 1; dx++) for (let dy = -1; dy <= 1; dy++) for (let dz = -1; dz <= 1; dz++) {
      for (const j of grid.get([cx + dx, cy + dy, cz + dz].join(",")) || []) {
        if (j === i || j === near(i, 1) || !res[j].n || !hPos[j] || dist(acc.ca, res[j].ca) >= cell) continue;
        const rON = dist(acc.o, res[j].n!), rCH = dist(acc.c, hPos[j]!), rOH = dist(acc.o, hPos[j]!), rCN = dist(acc.c, res[j].n!);
        const e = Math.min(rON, rCH, rOH, rCN) < 0.5 ? -9.9 : q * (1 / rON + 1 / rCH - 1 / rOH - 1 / rCN);
        if (e < -0.5) { hb.add(i * n + j); hbonds++; }
      }
    }
  }
  const has = (i: number, j: number) => i >= 0 && j >= 0 && hb.has(i * n + j);
  // α-helix: turn4(i-1) && turn4(i) → residues i..i+3 are H.
  const turn4 = (i: number) => i >= 0 && near(i, 4) >= 0 && has(i, i + 4);
  for (let i = 1; i < n; i++) if (turn4(i - 1) && turn4(i)) for (let k = i; k <= i + 3; k++) ss[k] = "H";
  // β-bridges.
  const bridges = new Set<string>();
  for (let i = 0; i < n; i++) {
    const ip = near(i, -1), inx = near(i, 1);
    if (ip < 0 || inx < 0) continue;
    for (let j = i + 1; j < n; j++) {
      if (segOf[j] === segOf[i] && j - i < 3) continue;
      const jp = near(j, -1), jn = near(j, 1);
      if (jp < 0 || jn < 0 || dist(res[i].ca, res[j].ca) >= cell) continue;
      if ((has(ip, j) && has(j, inx)) || (has(jp, i) && has(i, jn))) bridges.add(i + ",P," + j);
      if ((has(i, j) && has(j, i)) || (has(ip, jn) && has(jp, inx))) bridges.add(i + ",A," + j);
    }
  }
  for (const key of bridges) {
    const [si, type, sj] = key.split(",");
    const i = Number(si), j = Number(sj);
    const i2 = near(i, 1);
    const j2 = type === "P" ? near(j, 1) : near(j, -1);
    if (i2 >= 0 && j2 >= 0 && bridges.has(i2 + "," + type + "," + j2)) {
      for (const k of [i, i2, j, j2]) if (ss[k] !== "H") ss[k] = "E";
    }
  }
  return { ss, hbonds, usable };
}

export function assignSecondary(res: CartoonResidue[], segments: number[][], records: SecondaryRecords | null): SecondaryAssignment {
  let note = "";
  if (records && records.ranges.length) {
    const ss: SecondaryType[] = res.map((r) => {
      const hit = records.ranges.find((g) => g.chain === r.chain && r.seq >= g.start && r.seq <= g.end);
      return hit ? hit.type : "C";
    });
    if (ss.some((t) => t !== "C")) {
      return { ss, source: records.source === "pdb" ? "records-pdb" : "records-cif", hbonds: 0,
        note: records.source === "pdb" ? "文件自带 PDB HELIX/SHEET 记录" : "文件自带 mmCIF _struct_conf / _struct_sheet_range 记录" };
    }
    note = "文件有二级结构记录但未匹配到当前残基；";
  }
  const inferred = inferSecondary(res, segments);
  if (!inferred.usable) {
    return { ss: inferred.ss, source: "inferred-none", hbonds: 0, note: note + "推断：缺主链 N/C/O，无法判定，全部按 coil 绘制" };
  }
  return { ss: inferred.ss, source: "inferred-dssp", hbonds: inferred.hbonds,
    note: note + "推断：文件无二级结构记录，按简化 DSSP（主链 N–H···O=C 氢键能量，仅 α 螺旋与 β 梯）几何判定" };
}

export interface CartoonStyle {
  coilRadius: number; helixWidth: number; helixThickness: number;
  strandWidth: number; strandThickness: number; arrowWidth: number;
  samplesPerResidue: number; profileVertices: number;
}
// Widths are in Å before the stage's uniform display scale.
export const DEFAULT_CARTOON_STYLE: CartoonStyle = {
  coilRadius: 0.32, helixWidth: 1.9, helixThickness: 0.42, strandWidth: 1.9, strandThickness: 0.48,
  arrowWidth: 3.0, samplesPerResidue: 8, profileVertices: 12,
};

export interface CartoonMeshData {
  positions: Float32Array; colors: Float32Array; indices: Uint32Array;
  residueBySample: Int32Array; ringSize: number; samples: number; segment: number[];
}

// Pure geometry; the stage turns each result into one THREE.Mesh.
export function buildCartoonGeometry(
  res: CartoonResidue[], segments: number[][], ss: SecondaryType[],
  toDisplay: (p: Vec3) => THREE.Vector3, scale: number, colorOf: (residueIndex: number) => THREE.Color,
  style: CartoonStyle = DEFAULT_CARTOON_STYLE,
): CartoonMeshData[] {
  const out: CartoonMeshData[] = [];
  const S = style.samplesPerResidue, Q = style.profileVertices;
  const profile = (t: SecondaryType): [number, number] => t === "H" ? [style.helixWidth, style.helixThickness]
    : t === "E" ? [style.strandWidth, style.strandThickness] : [style.coilRadius * 2, style.coilRadius * 2];
  for (const seg of segments) {
    const m = seg.length;
    if (m < 2) continue;
    const points = seg.map((i) => toDisplay(res[i].ca));
    const curve = new THREE.CatmullRomCurve3(points, false, "centripetal");
    const guides: THREE.Vector3[] = [];
    seg.forEach((i, k) => {
      const r = res[i];
      let g = r.o ? toDisplay(r.o).sub(points[k]) : (guides[k - 1]?.clone() ?? new THREE.Vector3());
      if (k && g.dot(guides[k - 1]) < 0) g = g.negate();
      guides.push(g);
    });
    const samples = (m - 1) * S + 1;
    const positions = new Float32Array(samples * Q * 3), colors = new Float32Array(samples * Q * 3);
    const residueBySample = new Int32Array(samples);
    let prevN: THREE.Vector3 | null = null;
    for (let s = 0; s < samples; s++) {
      const local = s / S;
      const k = Math.min(Math.floor(local), m - 2);
      const f = local - k;
      const t = s / (samples - 1);
      const P = curve.getPoint(t), T = curve.getTangent(t).normalize();
      const g = guides[k].clone().lerp(guides[k + 1], f);
      let N = g.sub(T.clone().multiplyScalar(g.dot(T)));
      if (N.lengthSq() < 1e-10) N = prevN ? prevN.clone() : new THREE.Vector3(T.y, -T.x, 0).add(new THREE.Vector3(0, T.z, -T.y));
      N.normalize();
      if (prevN && N.dot(prevN) < 0) N.negate();
      prevN = N.clone();
      const B = new THREE.Vector3().crossVectors(T, N).normalize();
      const a = ss[seg[k]], b = ss[seg[k + 1]];
      let w: number, h: number;
      // Strand arrow head: flares at the last strand residue and tapers into the next state
      // (or to a coil-width point when the strand ends the segment).
      const arrowHere = a === "E" && b !== "E";
      const lastStrandEnd = a === "E" && b === "E" && k + 1 === m - 1;
      if (arrowHere || lastStrandEnd) {
        const end = profile(lastStrandEnd ? "C" : b);
        w = style.arrowWidth + (end[0] - style.arrowWidth) * f;
        h = style.strandThickness + (end[1] - style.strandThickness) * f;
      } else {
        const pa = profile(a), pb = profile(b);
        const e = f * f * (3 - 2 * f);
        w = pa[0] + (pb[0] - pa[0]) * e; h = pa[1] + (pb[1] - pa[1]) * e;
      }
      const owner = f < 0.5 ? seg[k] : seg[k + 1];
      residueBySample[s] = owner;
      const color = colorOf(owner);
      for (let v = 0; v < Q; v++) {
        const th = (v / Q) * Math.PI * 2;
        const cw = Math.cos(th) * w * 0.5 * scale, sh = Math.sin(th) * h * 0.5 * scale;
        const o = (s * Q + v) * 3;
        positions[o] = P.x + N.x * cw + B.x * sh;
        positions[o + 1] = P.y + N.y * cw + B.y * sh;
        positions[o + 2] = P.z + N.z * cw + B.z * sh;
        colors[o] = color.r; colors[o + 1] = color.g; colors[o + 2] = color.b;
      }
    }
    const indices = new Uint32Array((samples - 1) * Q * 6);
    let p = 0;
    for (let s = 0; s < samples - 1; s++) for (let v = 0; v < Q; v++) {
      const a = s * Q + v, b = s * Q + (v + 1) % Q, c = (s + 1) * Q + v, d = (s + 1) * Q + (v + 1) % Q;
      indices[p++] = a; indices[p++] = c; indices[p++] = b;
      indices[p++] = b; indices[p++] = c; indices[p++] = d;
    }
    out.push({ positions, colors, indices, residueBySample, ringSize: Q, samples, segment: seg });
  }
  return out;
}

export function secondaryCounts(ss: SecondaryType[]) {
  return { H: ss.filter((t) => t === "H").length, E: ss.filter((t) => t === "E").length, C: ss.filter((t) => t === "C").length };
}
