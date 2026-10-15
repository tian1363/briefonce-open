import { createContext, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ReactFlow, Background, Controls, Handle, MiniMap, Position } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import CanvasImagePreview, { CanvasPreviewContext } from "./CanvasImagePreview.jsx";

function CanvasNode({ data }) {
  return <div className={`workflow-node workflow-node-${data.kind}`}>
    <Handle type="target" position={Position.Left} className="workflow-handle" />
    <div className="workflow-node-head"><span>{data.step}</span><strong>{data.title}</strong></div>
    <div className="workflow-node-body">{data.content}</div>
    <Handle type="source" position={Position.Right} className="workflow-handle" />
  </div>;
}

export const CanvasToolbarContext = createContext(null);

const nodeTypes = { workflow: CanvasNode };

function savedPositions(key, legacyKey) {
  try { return JSON.parse(localStorage.getItem(key) || (legacyKey ? localStorage.getItem(legacyKey) : null) || "{}"); } catch { return {}; }
}

// Scene components provide their own nodes and edges; the board owns only layout.
export default function CanvasBoard({ userId, productId, sceneId, nodes, edges, galleryItems = [] }) {
  const toolbar = useContext(CanvasToolbarContext);
  const stage = useRef(null);
  const toolbarRef = useRef(null);
  const manualViewport = useRef(false);
  const [flow, setFlow] = useState(null);
  const [viewportSize, setViewportSize] = useState({ height: 600, toolbarHeight: 0 });
  useLayoutEffect(() => {
    const measure = () => {
      if (!stage.current?.getClientRects().length) return;
      const height = Math.max(280, window.innerHeight - stage.current.getBoundingClientRect().top - 12);
      const toolbarHeight = toolbarRef.current?.getBoundingClientRect().height || 0;
      setViewportSize(current => current.height === height && current.toolbarHeight === toolbarHeight ? current : {height,toolbarHeight});
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(stage.current);
    if (toolbarRef.current) observer.observe(toolbarRef.current);
    window.addEventListener("resize", measure);
    return () => { observer.disconnect(); window.removeEventListener("resize", measure); };
  }, []);

  const [preview, setPreview] = useState(null);
  function openPreview(src) {
    const visible = Array.from(stage.current?.querySelectorAll(".workflow-node-body img") || []).map((img) => ({ src: img.getAttribute("src"), alt: img.alt, filename: img.closest(".downloadable-image")?.querySelector("a[download]")?.getAttribute("download") || img.alt }));
    const images = [...visible, ...galleryItems].filter((item, index, all) => item.src && all.findIndex((candidate) => candidate.src === item.src) === index);
    const index = images.findIndex((item) => item.src === src);
    if (index >= 0) setPreview({ images, index });
  }
  const storageKey = `scriptagent:workflow-canvas:${userId}:${productId}:${sceneId}`;
  const legacyKey = sceneId === "pose" ? `scriptagent:workflow-canvas:${userId}:${productId}` : "";
  const [positions, setPositions] = useState(() => savedPositions(storageKey, legacyKey));
  const [measurements, setMeasurements] = useState({});
  const [layoutVersion, setLayoutVersion] = useState(0);
  useEffect(() => { try { localStorage.setItem(storageKey, JSON.stringify(positions)); } catch {} }, [storageKey, positions]);
  const flowNodes = useMemo(() => nodes.map((node) => ({ ...node, measured: measurements[node.id] || node.measured, position: positions[node.id] || node.position, type: "workflow", draggable: true, dragHandle: ".workflow-node-head", selectable: false })), [nodes, positions, measurements]);
  useEffect(() => {
    if (!flow || manualViewport.current || !flowNodes.length || flowNodes.some(node => !node.measured?.height)) return;
    const timer = setTimeout(() => {
      if (!manualViewport.current) flow.fitView({ padding: 0.08, minZoom: 0.05, maxZoom: 0.8, duration: 0 });
    }, 160);
    return () => clearTimeout(timer);
  }, [flow, flowNodes, viewportSize]);
  function resetLayout() {
    manualViewport.current = false;
    setPositions({});
    setLayoutVersion((version) => version + 1);
    try { localStorage.removeItem(storageKey); } catch {}
  }
  return <CanvasPreviewContext.Provider value={openPreview}><div ref={stage} className={`workflow-stage ${toolbar ? "with-toolbar" : ""}`} style={{height:viewportSize.height}} aria-label="工作流">{toolbar && <div ref={toolbarRef} className="workflow-canvas-toolbar nodrag nopan" role="group" aria-label="工作流工具栏">{toolbar}</div>}<button type="button" className="workflow-reset-view" onClick={resetLayout}>重置视图</button><div className="workflow-viewport" style={{top:toolbar ? viewportSize.toolbarHeight + 8 : 0}} onPointerDownCapture={()=>{manualViewport.current=true;}} onWheelCapture={()=>{manualViewport.current=true;}}><ReactFlow onInit={setFlow} onMoveStart={event=>{if(event)manualViewport.current=true;}} key={layoutVersion} nodes={flowNodes} edges={edges} nodeTypes={nodeTypes} autoPanOnNodeFocus={false} defaultViewport={{ x: 0, y: 0, zoom: 0.8 }} minZoom={0.05} maxZoom={1.5} onNodesChange={(changes) => {
    const moved = changes.filter((change) => change.type === "position" && change.position);
    if (moved.length) setPositions((current) => Object.fromEntries([...Object.entries(current), ...moved.map((change) => [change.id, change.position])]));
    // Preserve React Flow measurements across data updates. Dropping them makes
    // every keystroke hide/re-measure the node, which blurs its active input.
    const resized = changes.filter((change) => change.type === "dimensions" && change.dimensions);
    if (resized.length) setMeasurements((current) => {
      const changed = resized.filter((change) => current[change.id]?.width !== change.dimensions.width || current[change.id]?.height !== change.dimensions.height);
      return changed.length ? { ...current, ...Object.fromEntries(changed.map((change) => [change.id, change.dimensions])) } : current;
    });
  }}><Background gap={22} size={1} /><MiniMap pannable zoomable /><Controls fitViewOptions={{padding:0.08,minZoom:0.05,maxZoom:0.8}} /></ReactFlow></div></div>{preview && <CanvasImagePreview images={preview.images} initialIndex={preview.index} onClose={() => setPreview(null)} />}</CanvasPreviewContext.Provider>;
}
