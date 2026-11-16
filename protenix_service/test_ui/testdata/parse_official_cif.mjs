import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import assert from "node:assert/strict";

const here = path.dirname(fileURLToPath(import.meta.url));
const html = fs.readFileSync(path.join(here, "..", "flow_debug.html"), "utf8");
const begin = html.indexOf("// PARSE_CIF_BEGIN");
const end = html.indexOf("// PARSE_CIF_END");
if (begin < 0 || end < 0 || end <= begin) {
  throw new Error("PARSE_CIF markers missing");
}
const snippet = html.slice(begin, end);
if (snippet.includes("Math.random()") || snippet.includes("fakeProgress")) {
  throw new Error("parser snippet contains banned tokens");
}
const { parseCIF, backboneSegments, centerAtoms } = vm.runInNewContext(snippet + "\n({ parseCIF, backboneSegments, centerAtoms });");

const cif = fs.readFileSync(path.join(here, "sqlite_smoke_sample_0.cif"), "utf8");
const atoms = parseCIF(cif);
const first = atoms[0];
const last = atoms[atoms.length - 1];
const ca = atoms.filter((atom) => atom.name === "CA");
const checks = [
  ["count", atoms.length, 168],
  ["first.element", first?.element, "N"],
  ["first.name", first?.name, "N"],
  ["first.seq", first?.seq, 1],
  ["first.x", first?.x, 840.8047],
  ["first.y", first?.y, 189.25467],
  ["first.z", first?.z, -336.60333],
  ["last.element", last?.element, "O"],
  ["last.name", last?.name, "OXT"],
  ["last.seq", last?.seq, 20],
  ["last.x", last?.x, -916.35425],
  ["last.y", last?.y, 565.70996],
  ["last.z", last?.z, -280.20642],
  ["CA count", ca.length, 20],
];
const failed = [];
for (const [label, got, want] of checks) {
  const ok = typeof want === "number" && !Number.isInteger(want)
    ? Math.abs(Number(got) - want) < 1e-6
    : Object.is(got, want);
  if (!ok) failed.push(`${label}: got ${got} want ${want}`);
}
// A classic PDB-only parse of these ATOM-prefixed mmCIF rows must not be added on top.
if (atoms.length !== 168) failed.push("official mmCIF must yield exactly 168 atoms, no PDB-duplicate mix");
if (failed.length) {
  console.error(failed.join("\n"));
  process.exit(1);
}
const pdb = "ATOM      1  N   ALA A   1      11.104  13.207   9.876  1.00  0.00           N  \nATOM      2  CA  ALA A   1      12.201  12.300   9.500  1.00  0.00           C  \n";
const pdbAtoms = parseCIF(pdb);
if (pdbAtoms.length !== 2 || pdbAtoms[0].element !== "N" || pdbAtoms[1].name !== "CA") {
  console.error("classic PDB parse failed", pdbAtoms);
  process.exit(1);
}
if (Math.abs(pdbAtoms[0].x - 11.104) > 1e-6 || Math.abs(pdbAtoms[0].y - 13.207) > 1e-6) {
  console.error("classic PDB coordinates failed", pdbAtoms[0]);
  process.exit(1);
}
console.log("parse_official_cif ok 168 atoms, 20 CA; classic PDB 2 atoms");

// A two-step smoke output is real inference, but not a validated physical structure.
const smokeBackbone = backboneSegments(atoms);
assert.equal(smokeBackbone.links.length, 0);
assert.equal(smokeBackbone.rejected, 19);
const base = {element: "C", name: "CA", residue: "ALA", model: "1", chain: "A", segment: 0, insertion: "", x: 0, y: 0, z: 0, seq: 1};
const next = {...base, x: 3.8, seq: 2};
assert.equal(backboneSegments([base, next]).links.length, 1);
for (const override of [{model:"2"}, {chain:"B"}, {segment:1}, {insertion:"A"}, {seq:4}, {x:10}, {residue:"CA",element:"CA"}]) {
  assert.equal(backboneSegments([base, {...next, ...override}]).links.length, 0, JSON.stringify(override));
}
const centered = centerAtoms([base, next]);
assert.equal(centered[0].x, -1.9);
assert.equal(centered[0].ox, 0);
assert.equal(centered[1].x - centered[0].x, 3.8);
const multiPDB = "MODEL        1\n" + pdb + "ENDMDL\nMODEL        2\n" + pdb + "ENDMDL\n";
const multi = parseCIF(multiPDB);
assert.equal(multi.length, 4);
assert.equal(multi[0].model, "1"); assert.equal(multi[2].model, "2");
const tinyCIF = `data_test
loop_
_atom_site.group_PDB
_atom_site.type_symbol
_atom_site.label_atom_id
_atom_site.label_comp_id
_atom_site.label_seq_id
_atom_site.label_asym_id
_atom_site.Cartn_x
_atom_site.Cartn_y
_atom_site.Cartn_z
_atom_site.pdbx_PDB_model_num
_atom_site.label_alt_id
_atom_site.occupancy
ATOM C 'CA' ALA 1 AA 0 0 0 1 A 1
ATOM C 'CA' ALA 1 AA 1 0 0 1 B 1
ATOM C 'CA' ALA 2 AA 3.8 0 0 1 . 1
ATOM C 'CA' ALA 1 AA 2 0 0 2 . 1
ATOM C 'CA' ALA 2 AA 3 0 0 2 . 0
#
`;
const parsed = parseCIF(tinyCIF);
assert.equal(parsed.length, 3);
assert.equal(parsed[0].x, 0); assert.equal(parsed[0].name, "CA"); assert.equal(parsed[0].chain, "AA");
assert.equal(backboneSegments(parsed).links.length, 1);
console.log("structure topology ok: no inter-model/chain/TER/gap/long-distance/calcium bridges; primary alt; raw coordinates preserved");
