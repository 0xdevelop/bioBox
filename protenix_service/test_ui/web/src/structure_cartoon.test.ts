import { test, expect } from "bun:test";
import * as THREE from "three";
import {
  assignSecondary, buildCartoonGeometry, collectResidues, inferSecondary, parseSecondaryRecords, splitSegments,
  type CartoonAtom, type Vec3,
} from "./structure_cartoon";

// NeRF placement with standard backbone geometry (Engh & Huber); used only to build test chains.
function place(a: Vec3, b: Vec3, c: Vec3, length: number, angleDeg: number, torsionDeg: number): Vec3 {
  const v = new THREE.Vector3(...c).sub(new THREE.Vector3(...b)).normalize();
  const n = new THREE.Vector3(...b).sub(new THREE.Vector3(...a)).cross(v).normalize();
  const m = new THREE.Vector3().crossVectors(n, v);
  const th = (angleDeg * Math.PI) / 180, ch = (torsionDeg * Math.PI) / 180;
  const d = new THREE.Vector3(...c)
    .add(v.multiplyScalar(-length * Math.cos(th)))
    .add(m.multiplyScalar(length * Math.sin(th) * Math.cos(ch)))
    .add(n.multiplyScalar(length * Math.sin(th) * Math.sin(ch)));
  return [d.x, d.y, d.z];
}
function backbone(count: number, phi: number, psi: number, chain = "A", firstSeq = 1): CartoonAtom[] {
  let N: Vec3 = [0, 0, 0], CA: Vec3 = [1.458, 0, 0];
  let C: Vec3 = [1.458 - 1.525 * Math.cos((111.2 * Math.PI) / 180), 1.525 * Math.sin((111.2 * Math.PI) / 180), 0];
  const atoms: CartoonAtom[] = [];
  const add = (name: string, element: string, p: Vec3, seq: number) => atoms.push({ x: p[0], y: p[1], z: p[2], element, name, chain, residue: "ALA", seq, model: "1", segment: 0, insertion: "" });
  for (let i = 0; i < count; i++) {
    const nextN = place(N, CA, C, 1.329, 116.2, psi);
    const O = place(nextN, CA, C, 1.231, 120.5, 180);
    add("N", "N", N, firstSeq + i); add("CA", "C", CA, firstSeq + i); add("C", "C", C, firstSeq + i); add("O", "O", O, firstSeq + i);
    const nextCA = place(CA, C, nextN, 1.458, 121.7, 180);
    const nextC = place(C, nextN, nextCA, 1.525, 111.2, phi);
    N = nextN; CA = nextCA; C = nextC;
  }
  return atoms;
}

test("records are read verbatim from mmCIF loops, key-value blocks and PDB HELIX/SHEET", () => {
  const cif = `data_x
loop_
_struct_conf.conf_type_id
_struct_conf.id
_struct_conf.beg_label_asym_id
_struct_conf.beg_label_seq_id
_struct_conf.end_label_asym_id
_struct_conf.end_label_seq_id
HELX_P HELX_P1 A 3 A 9
TURN_P TURN_P1 A 10 A 12
#
_struct_sheet_range.sheet_id A
_struct_sheet_range.id 1
_struct_sheet_range.beg_label_asym_id B
_struct_sheet_range.beg_label_seq_id 20
_struct_sheet_range.end_label_asym_id B
_struct_sheet_range.end_label_seq_id 24
#`;
  expect(parseSecondaryRecords(cif)).toEqual({ source: "cif", ranges: [
    { chain: "A", start: 3, end: 9, type: "H" }, { chain: "B", start: 20, end: 24, type: "E" }] });
  const pdb = "HELIX    1   1 ALA A    5  ALA A   12  1                                   8    \n" +
              "SHEET    1   A 2 ALA B  30  ALA B  33  0                                        \n";
  expect(parseSecondaryRecords(pdb)).toEqual({ source: "pdb", ranges: [
    { chain: "A", start: 5, end: 12, type: "H" }, { chain: "B", start: 30, end: 33, type: "E" }] });
  expect(parseSecondaryRecords("data_y\nloop_\n_atom_site.id\n1\n").source).toBe("none");
});

test("simplified DSSP finds an ideal α-helix and nothing in an extended chain", () => {
  const helix = collectResidues(backbone(16, -57, -47));
  const hs = inferSecondary(helix, splitSegments(helix).segments);
  expect(splitSegments(helix).breaks).toEqual([]);
  expect(hs.ss.slice(1, 13).every((t) => t === "H")).toBe(true);
  expect(hs.ss[0]).toBe("C");
  const strand = collectResidues(backbone(12, -120, 130));
  const es = inferSecondary(strand, splitSegments(strand).segments);
  expect(es.ss.every((t) => t === "C")).toBe(true);
});

test("records win over inference and inference is labelled; no backbone O means no guess", () => {
  const res = collectResidues(backbone(16, -57, -47));
  const { segments } = splitSegments(res);
  const fromRecords = assignSecondary(res, segments, { source: "cif", ranges: [{ chain: "A", start: 2, end: 4, type: "E" }] });
  expect(fromRecords.source).toBe("records-cif");
  expect(fromRecords.ss.filter((t) => t === "E").length).toBe(3);
  expect(fromRecords.ss.includes("H")).toBe(false);
  const unmatched = assignSecondary(res, segments, { source: "cif", ranges: [{ chain: "Z", start: 2, end: 4, type: "E" }] });
  expect(unmatched.source).toBe("inferred-dssp");
  expect(unmatched.note).toContain("推断");
  const caOnly = collectResidues(backbone(10, -57, -47).filter((a) => a.name === "CA"));
  const none = assignSecondary(caOnly, splitSegments(caOnly).segments, null);
  expect(none.source).toBe("inferred-none");
  expect(none.ss.every((t) => t === "C")).toBe(true);
});

test("ribbons break at Cα gaps, numbering gaps and chain changes and pass through every Cα", () => {
  const a = backbone(6, -57, -47, "A", 1);
  const far = backbone(6, -57, -47, "A", 7).map((x) => ({ ...x, x: x.x + 40 })); // continuous numbering, 40 Å away
  const gap = backbone(4, -57, -47, "A", 20).map((x) => ({ ...x, x: x.x + 80 }));
  const other = backbone(3, -57, -47, "B", 1).map((x) => ({ ...x, y: x.y + 40 }));
  const res = collectResidues([...a, ...far, ...gap, ...other]);
  const { segments, breaks } = splitSegments(res);
  expect(segments.map((s) => s.length)).toEqual([6, 6, 4, 3]);
  expect(breaks.length).toBe(2);
  expect(breaks[0].reason).toContain("Cα 距离");
  expect(breaks[1].reason).toBe("残基序号不连续");
  const ss = res.map(() => "C" as const);
  const id = (p: Vec3) => new THREE.Vector3(...p);
  const meshes = buildCartoonGeometry(res, segments, ss, id, 1, () => new THREE.Color("#ffffff"));
  expect(meshes.length).toBe(4);
  for (const mesh of meshes) {
    mesh.segment.forEach((r, k) => {
      const s = k * 8, Q = mesh.ringSize;
      const center = [0, 1, 2].map((d) => { let v = 0; for (let q = 0; q < Q; q++) v += mesh.positions[(s * Q + q) * 3 + d]; return v / Q; });
      expect(Math.hypot(center[0] - res[r].ca[0], center[1] - res[r].ca[1], center[2] - res[r].ca[2])).toBeLessThan(1e-4);
    });
  }
  // No triangle joins two segments: every index stays inside its own mesh.
  for (const mesh of meshes) expect(Math.max(...mesh.indices)).toBeLessThan(mesh.samples * mesh.ringSize);
});
