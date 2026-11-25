import { test, expect } from "bun:test";
import { displayTransform, type StructureAtom } from "./structure_stage";
const atom = (x: number, y = 0, z = 0) => ({x, y, z, element:"C", name:"CA", chain:"A", residue:"ALA", seq:1, model:"1"}) as StructureAtom;
test("display transform preserves pairwise geometry and original coordinates", () => {
  const atoms = [atom(100, 200, -300), atom(103.8, 200, -300)];
  const original = JSON.stringify(atoms);
  const {center, scale} = displayTransform(atoms);
  expect(center.x).toBeCloseTo(101.9);
  expect(center.y).toBe(200);
  expect(center.z).toBe(-300);
  expect(3.8 * scale).toBeCloseTo(26);
  expect(JSON.stringify(atoms)).toBe(original);
});
test("empty, invalid and excessive structures have explicit bounds", () => {
  expect(displayTransform([]).scale).toBe(13);
  expect(() => displayTransform([atom(NaN)])).toThrow("非有限");
  expect(() => displayTransform(Array(100001).fill(atom(1)))).toThrow("100000");
});
