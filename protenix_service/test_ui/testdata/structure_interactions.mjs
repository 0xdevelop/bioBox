import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import path from "node:path";
import {fileURLToPath} from "node:url";
const here = path.dirname(fileURLToPath(import.meta.url));
const html = fs.readFileSync(path.join(here, "..", "flow_debug.html"), "utf8");
let script = html.slice(html.indexOf("<script>") + 8, html.indexOf("</script>"));
script = script.replace("    bindStage();", "    globalThis.flow = { state, resetTaskView, loadArtifacts, fetchArtifact, stageAtoms, updateStructureView, parseCIF };\n    bindStage();");
class Element {
  value = ""; textContent = ""; hidden = false; disabled = false; style = {}; children = []; events = {}; files = [];
  set innerHTML(value) { this.children = []; }
  get innerHTML() { return ""; }
  get options() { return this.children; }
  replaceChildren(...children) { this.children = children; this.value = children[0]?.value || ""; }
  appendChild(child) { this.children.push(child); if (this.children.length === 1 && child.value != null) this.value = child.value; }
  addEventListener(name, fn) { this.events[name] = fn; }
  setAttribute() {} remove() {} getBoundingClientRect() { return {left:0,top:0,width:900,height:720}; }
}
const elements = new Map();
const get = (id) => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
for (const [id, value] of [["transport","jsonrpc"], ["rpc-url","/rpc"], ["api-key","test-only"], ["structure-view","atoms"], ["structure-color","chain"]]) get(id).value = value;
const cif = fs.readFileSync(path.join(here, "sqlite_smoke_sample_0.cif"), "utf8");
const pending = [];
const response = (payload) => ({ok:true, status:200, text:async () => JSON.stringify({result:{isError:false,content:[{type:"text",text:JSON.stringify(payload)}]}})});
const context = vm.createContext({console, performance, matchMedia:() => ({matches:false}),
  document:{ hidden:true, getElementById:get, createElement:() => new Element(), querySelectorAll:() => [] },
  window:{ addEventListener(){} }, requestAnimationFrame:() => 1, cancelAnimationFrame(){},
  setInterval:() => 1, clearInterval(){}, atob:(v) => Buffer.from(v,"base64").toString("binary"),
  fetch:async (url, options) => {
    if (url === "/api/config") return {ok:false};
    const call = JSON.parse(options.body).params;
    if (call.name === "task.artifacts") return response({artifacts:[{path:"sample_0.cif",kind:"cif"},{path:"sample_1.cif",kind:"cif"}]});
    if (call.name === "task.artifact") return new Promise((resolve, reject) => pending.push({path:call.arguments.path,resolve,reject}));
    throw new Error("unexpected call " + call.name);
  }
});
vm.runInContext(script, context);
const f = context.flow;
const settle = async () => { for (let i=0;i<12;i++) await Promise.resolve(); };
const artifact = (data=cif) => response({encoding:"base64",content:Buffer.from(data).toString("base64"),kind:"cif"});
f.resetTaskView("A");
const listing = f.loadArtifacts("A"); await settle();
assert.equal(get("structure-select").options.length,2);
assert.equal(pending.length,1);
const second = f.fetchArtifact("A","sample_1.cif","cif"); await settle();
pending[0].resolve(artifact()); await listing;
assert.equal(f.state.atoms.length,0,"older sample must not overwrite newer selection");
pending[1].resolve(artifact()); await second;
assert.equal(f.state.stagePath,"sample_1.cif"); assert.equal(f.stageAtoms().length,168);
assert.ok(get("structure-summary").textContent.includes("19 处相邻"),"real smoke geometry warning");
assert.equal(get("structure-model").options.length,1);
const stale = f.fetchArtifact("A","late.cif","cif"); await settle();
f.resetTaskView("B"); pending[2].resolve(artifact()); await stale;
assert.equal(f.state.atoms.length,0); assert.equal(get("structure-select").options.length,0);
const failed = f.fetchArtifact("B","bad.cif","cif"); await settle(); pending[3].reject(new Error("read failure"));
await assert.rejects(failed,/read failure/); assert.equal(f.state.atoms.length,0);
f.state.atoms = [
 {x:0,y:0,z:0,element:"C",name:"CA",residue:"ALA",seq:1,model:"1",chain:"A",segment:0,insertion:""},
 {x:3.8,y:0,z:0,element:"C",name:"CA",residue:"ALA",seq:2,model:"1",chain:"A",segment:0,insertion:""},
 {x:0,y:0,z:0,element:"C",name:"CA",residue:"ALA",seq:1,model:"2",chain:"B",segment:0,insertion:""}
];
f.state.stageTaskId="B"; f.state.stagePath="models.pdb"; f.updateStructureView(true);
assert.equal(f.stageAtoms().length,2);
get("structure-model").value="2"; f.updateStructureView(false); assert.equal(f.stageAtoms().length,1);
get("structure-chain").value="B"; f.updateStructureView(false); assert.equal(f.stageAtoms().length,1);
let snapshot;
f.state.three={update:(s) => snapshot=s, clear(){},dispose(){}}; f.updateStructureView(false);
assert.equal(snapshot.atoms.length,1); assert.equal(snapshot.links.length,0); assert.equal(snapshot.traceIndices.length,1);
console.log("structure interactions ok: sample/model/chain selection, real geometry warning, late response isolation, failure clear, Three snapshot; VM DOM test, not browser evidence");
