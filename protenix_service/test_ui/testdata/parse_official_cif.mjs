import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

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
const moduleText = snippet.replace("// PARSE_CIF_BEGIN", "") + "\nexport { parseCIF };\n";
const tmp = path.join(here, ".parse_cif_extract.mjs");
fs.writeFileSync(tmp, moduleText);
const { parseCIF } = await import(pathToFileURL(tmp).href);
fs.unlinkSync(tmp);

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
