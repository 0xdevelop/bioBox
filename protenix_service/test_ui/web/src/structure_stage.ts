import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";

export interface StructureAtom {
  x: number; y: number; z: number; element: string; name: string;
  chain: string; residue: string; seq: number | null; model: string;
}
export interface StructureSnapshot {
  atoms: StructureAtom[];
  links: [number, number][];
  colors: string[];
  trace: boolean;
  traceIndices: number[];
  label: string;
  rotating: boolean;
}

// One uniform display transform only. Original Å coordinates remain available for picking.
export function displayTransform(atoms: StructureAtom[]) {
  if (atoms.some((a) => ![a.x, a.y, a.z].every(Number.isFinite))) throw new Error("结构含非有限坐标");
  if (atoms.length > 100000) throw new Error("交互舞台最多 100000 原子；请先选择单链，或使用兼容视图");
  const center = new THREE.Vector3();
  for (const a of atoms) center.add(new THREE.Vector3(a.x, a.y, a.z).multiplyScalar(1 / atoms.length));
  let radius = 1;
  for (const a of atoms) radius = Math.max(radius, new THREE.Vector3(a.x, a.y, a.z).distanceTo(center));
  return { center, scale: 13 / radius };
}

function release(group: THREE.Group) {
  group.traverse((object) => {
    const mesh = object as THREE.Mesh;
    if (mesh.geometry) mesh.geometry.dispose();
    if (mesh.material) for (const material of Array.isArray(mesh.material) ? mesh.material : [mesh.material]) material.dispose();
    if (object instanceof THREE.InstancedMesh) object.dispose();
  });
  group.clear();
}

export function createStructureStage(host: HTMLElement, onError: (message: string) => void) {
  const scene = new THREE.Scene();
  scene.background = new THREE.Color("#030a0e");
  scene.fog = new THREE.FogExp2("#030a0e", 0.009);
  const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 240);
  const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: "high-performance" });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.35;
  renderer.domElement.setAttribute("aria-label", "Three.js 真实坐标三维结构；拖拽旋转，滚轮缩放，右键平移");
  renderer.domElement.tabIndex = 0;
  host.replaceChildren(renderer.domElement);
  const info = document.createElement("div");
  info.className = "three-hud";
  info.setAttribute("aria-live", "polite");
  host.appendChild(info);
  const tooltip = document.createElement("div");
  tooltip.className = "three-tooltip";
  host.appendChild(tooltip);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.075;
  controls.autoRotateSpeed = 0.45;
  controls.minDistance = 6;
  controls.maxDistance = 110;
  controls.enablePan = true;
  controls.listenToKeyEvents(renderer.domElement);
  const molecule = new THREE.Group(), decor = new THREE.Group();
  scene.add(molecule, decor);
  scene.add(new THREE.HemisphereLight(0xb7fdeb, 0x13223b, 2.2));
  const key = new THREE.DirectionalLight(0xd2fff2, 3.5); key.position.set(15, 25, 20); scene.add(key);
  const rim = new THREE.PointLight(0x64aaff, 95, 100, 1.5); rim.position.set(-20, 10, -8); scene.add(rim);
  // Instrument rings are presentation only. They never represent molecular coordinates or progress.
  const rotor = new THREE.Group(); decor.add(rotor);
  for (let i = 0; i < 4; i++) {
    const geometry = new THREE.BufferGeometry();
    const points: number[] = [];
    const radius = 17 + i * 2.1;
    for (let k = 0; k <= 192; k++) {
      const t = k / 192 * Math.PI * (i === 1 ? 1.65 : 2);
      points.push(Math.cos(t) * radius, -15 + i * 0.12, Math.sin(t) * radius);
    }
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(points, 3));
    rotor.add(new THREE.Line(geometry, new THREE.LineBasicMaterial({ color: i % 2 ? 0x4085b3 : 0x36ba8e, transparent: true, opacity: i === 0 ? 0.5 : 0.2 })));
  }
  const spokes: number[] = [];
  for (let k = 0; k < 72; k++) {
    const t = k / 72 * Math.PI * 2;
    for (const r of [20.5, k % 6 ? 21 : 22.1]) spokes.push(Math.cos(t) * r, -14.95, Math.sin(t) * r);
  }
  const spokeGeometry = new THREE.BufferGeometry(); spokeGeometry.setAttribute("position", new THREE.Float32BufferAttribute(spokes, 3));
  decor.add(new THREE.LineSegments(spokeGeometry, new THREE.LineBasicMaterial({ color: 0x5eddb2, transparent: true, opacity: 0.35 })));
  const dust: number[] = [];
  for (let i = 0; i < 300; i++) {
    const angle = i * 2.399963, radius = 28 + (i % 27) * 0.65;
    dust.push(Math.cos(angle) * radius, ((i * 17) % 67) - 31, Math.sin(angle) * radius);
  }
  const dustGeometry = new THREE.BufferGeometry(); dustGeometry.setAttribute("position", new THREE.Float32BufferAttribute(dust, 3));
  decor.add(new THREE.Points(dustGeometry, new THREE.PointsMaterial({ color: 0x70d1c8, size: 0.055, transparent: true, opacity: 0.45, depthWrite: false })));
  let snapshot: StructureSnapshot | null = null;
  let atomMesh: THREE.InstancedMesh | null = null;
  let meshAtoms: StructureAtom[] = [];
  let disposed = false, rotating = true, visible = true;
  let previous = performance.now(), frame = 0;
  let pointer: THREE.Vector2 | null = null;
  const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)");
  const raycaster = new THREE.Raycaster();
  const status = (text: string) => { info.textContent = text; };
  function reset() {
    camera.position.set(23, 17, 36); controls.target.set(0, 0, 0); controls.update(); controls.saveState();
  }
  reset();
  function clear() {
    release(molecule); atomMesh = null; meshAtoms = []; snapshot = null;
    tooltip.textContent = ""; pointer = null;
    status("等待真实结构 · NO MOLECULE LOADED · 装饰环不是计算进度");
  }
  function update(next: StructureSnapshot) {
    clear();
    const { center, scale } = displayTransform(next.atoms);
    if (next.links.some(([i, j]) => !next.atoms[i] || !next.atoms[j] || i === j)) throw new Error("无效 Cα 连接索引");
    snapshot = next; rotating = next.rotating;
    const caIndices = new Set(next.traceIndices);
    const indices = next.atoms.map((_, i) => i).filter((i) => !next.trace || caIndices.has(i));
    if (!next.atoms.length) return;
    const positions = next.atoms.map((a) => new THREE.Vector3(a.x, a.y, a.z).sub(center).multiplyScalar(scale));
    if (indices.length) {
      atomMesh = new THREE.InstancedMesh(new THREE.SphereGeometry(1, 12, 8), new THREE.MeshStandardMaterial({ roughness: 0.26, metalness: 0.28 }), indices.length);
      const matrix = new THREE.Matrix4();
      const quaternion = new THREE.Quaternion();
      indices.forEach((index, instance) => {
        const radius = next.trace ? 0.22 : (next.atoms[index].element === "H" ? 0.13 : 0.24);
        matrix.compose(positions[index], quaternion, new THREE.Vector3(radius, radius, radius));
        atomMesh!.setMatrixAt(instance, matrix);
        atomMesh!.setColorAt(instance, new THREE.Color(next.colors[index] || "#9feaca"));
      });
      atomMesh.instanceMatrix.needsUpdate = true;
      if (atomMesh.instanceColor) atomMesh.instanceColor.needsUpdate = true;
      atomMesh.computeBoundingSphere(); molecule.add(atomMesh);
      meshAtoms = indices.map((i) => next.atoms[i]);
    }
    if (next.links.length) {
      const links = new THREE.InstancedMesh(new THREE.CylinderGeometry(next.trace ? 0.1 : 0.055, next.trace ? 0.1 : 0.055, 1, 8), new THREE.MeshStandardMaterial({ roughness: 0.32, metalness: 0.2 }), next.links.length);
      const yAxis = new THREE.Vector3(0, 1, 0), matrix = new THREE.Matrix4();
      next.links.forEach(([i, j], instance) => {
        if (!positions[i] || !positions[j]) throw new Error("无效 Cα 连接索引");
        const delta = positions[j].clone().sub(positions[i]);
        const length = delta.length();
        matrix.compose(positions[i].clone().add(positions[j]).multiplyScalar(0.5), new THREE.Quaternion().setFromUnitVectors(yAxis, delta.clone().normalize()), new THREE.Vector3(1, length, 1));
        links.setMatrixAt(instance, matrix); links.setColorAt(instance, new THREE.Color(next.colors[i]));
      });
      links.instanceMatrix.needsUpdate = true;
      if (links.instanceColor) links.instanceColor.needsUpdate = true;
      links.computeBoundingSphere(); molecule.add(links);
    }
    reset();
    status(next.label + " · " + next.atoms.length + " ATOMS / " + next.links.length + " Cα LINKS · 最终结构，非物理折叠 · 球径仅为显示符号");
  }
  function resize() {
    if (disposed) return;
    const w = Math.max(host.clientWidth, 1), h = Math.max(host.clientHeight, 1);
    renderer.setSize(w, h); camera.aspect = w / h; camera.updateProjectionMatrix();
  }
  const observer = new ResizeObserver(resize); observer.observe(host); resize();
  function move(event: PointerEvent) {
    const box = renderer.domElement.getBoundingClientRect();
    pointer = new THREE.Vector2((event.clientX - box.left) / box.width * 2 - 1, -(event.clientY - box.top) / box.height * 2 + 1);
  }
  const leave = () => { pointer = null; tooltip.textContent = ""; };
  const pauseOnInteraction = () => { rotating = false; host.dispatchEvent(new CustomEvent("structure-interaction")); };
  const contextLost = (event: Event) => { event.preventDefault(); if (!disposed) onError("WebGL 上下文丢失，请重试三维舞台或切换兼容视图"); };
  renderer.domElement.addEventListener("pointermove", move);
  renderer.domElement.addEventListener("pointerleave", leave);
  renderer.domElement.addEventListener("webglcontextlost", contextLost);
  controls.addEventListener("start", pauseOnInteraction);
  function render(now: number) {
    if (disposed) return;
    frame = requestAnimationFrame(render);
    const elapsedMs = now - previous;
    const delta = Math.min(elapsedMs / 1000, 0.05); previous = now;
    if (!visible || document.hidden) return;
    controls.autoRotate = rotating && !reducedMotion.matches && !!snapshot;
    controls.update(delta);
    if (rotating && !reducedMotion.matches) rotor.rotation.y += delta * 0.035;
    if (pointer && atomMesh) {
      raycaster.setFromCamera(pointer, camera);
      const hit = raycaster.intersectObject(atomMesh, false)[0];
      const a = hit?.instanceId == null ? null : meshAtoms[hit.instanceId];
      tooltip.textContent = a ? `链 ${a.chain || "(空)"} / ${a.residue} ${a.seq ?? "?"} / ${a.name} (${a.element}) · ${a.x.toFixed(3)}, ${a.y.toFixed(3)}, ${a.z.toFixed(3)} Å · 原始产物坐标` : "";
    }
    renderer.render(scene, camera);
    host.dataset.drawCalls = String(renderer.info.render.calls);
    host.dataset.geometries = String(renderer.info.memory.geometries);
    host.dataset.frameMs = elapsedMs.toFixed(2);
  }
  clear(); frame = requestAnimationFrame(render);
  return {
    update, clear, reset,
    pause(paused: boolean) { rotating = !paused; },
    setVisible(value: boolean) { visible = value; resize(); },
    dispose() {
      if (disposed) return;
      disposed = true; cancelAnimationFrame(frame); observer.disconnect();
      controls.removeEventListener("start", pauseOnInteraction); controls.dispose();
      renderer.domElement.removeEventListener("pointermove", move); renderer.domElement.removeEventListener("pointerleave", leave);
      renderer.domElement.removeEventListener("webglcontextlost", contextLost);
      release(molecule); release(decor); renderer.dispose(); renderer.forceContextLoss(); host.replaceChildren();
    },
  };
}
