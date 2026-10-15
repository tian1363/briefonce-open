import { Children, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown } from "lucide-react";
import "./CanvasSelect.css";

export default function CanvasSelect({ children, value = "", onChange, className = "", disabled = false, getOptionImage, ...props }) {
  const options = Children.toArray(children).flat().filter(Boolean).map(child => ({ value: String(child.props.value ?? child.props.children), label: child.props.children, disabled: child.props.disabled }));
  const selected = options.findIndex(option => option.value === String(value));
  const trigger = useRef(null), popup = useRef(null);
  const id = useId();
  const [open, setOpen] = useState(false), [active, setActive] = useState(0), [position, setPosition] = useState(null);
  const [label, setLabel] = useState("");
  useEffect(() => {
    const parent = trigger.current?.closest("label");
    setLabel(parent ? [...parent.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join("").trim() : "");
  }, []);
  useLayoutEffect(() => {
    if (!open) return;
    const update = () => {
      const rect = trigger.current.getBoundingClientRect();
      const below = window.innerHeight - rect.bottom - 12, above = rect.top - 12;
      const height = Math.min(320, Math.max(below, above));
      const width = Math.min(Math.max(rect.width, 200), window.innerWidth - 24);
      setPosition({ left: Math.max(12, Math.min(rect.left, window.innerWidth - width - 12)), width, maxHeight: height, ...(below >= Math.min(320, above) ? { top: rect.bottom + 6 } : { bottom: window.innerHeight - rect.top + 6 }) });
    };
    const outside = event => { if (!trigger.current?.contains(event.target) && !popup.current?.contains(event.target)) setOpen(false); };
    const scroll = event => { if (!popup.current?.contains(event.target)) setOpen(false); };
    update();
    document.addEventListener("pointerdown", outside, true);
    window.addEventListener("resize", update);
    window.addEventListener("scroll", scroll, true);
    return () => { document.removeEventListener("pointerdown", outside, true); window.removeEventListener("resize", update); window.removeEventListener("scroll", scroll, true); };
  }, [open]);
  useEffect(() => { if (open) popup.current?.querySelector(`[data-option-index="${active}"]`)?.scrollIntoView({ block: "nearest" }); }, [active, open, position]);
  function choose(index) {
    const option = options[index];
    if (!option || option.disabled) return;
    onChange?.({ target: { value: option.value } });
    setOpen(false);
    trigger.current?.focus();
  }
  function keyDown(event) {
    event.stopPropagation();
    if (event.key === "Tab") { setOpen(false); return; }
    if (event.key === "Escape") { event.preventDefault(); setOpen(false); return; }
    if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
      event.preventDefault();
      const enabled = options.map((o, i) => o.disabled ? -1 : i).filter(i => i >= 0);
      if (!enabled.length) return;
      if (!open) { setActive(selected >= 0 ? selected : enabled[0]); setOpen(true); return; }
      const index = enabled.indexOf(active), direction = event.key === "ArrowUp" ? -1 : 1;
      setActive(event.key === "Home" ? enabled[0] : event.key === "End" ? enabled.at(-1) : enabled[(index + direction + enabled.length) % enabled.length]);
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (open) choose(active); else { setActive(Math.max(0, selected)); setOpen(true); }
    }
  }
  return <>
    <button {...props} ref={trigger} type="button" role="combobox" aria-label={props["aria-label"] || label || undefined} aria-controls={open ? id : undefined} aria-expanded={open} aria-haspopup="listbox" aria-activedescendant={open ? `${id}-${active}` : undefined} disabled={disabled} className={`canvas-select-trigger nodrag nopan ${className}`} onPointerDown={event => event.stopPropagation()} onKeyDown={keyDown} onClick={event => { event.stopPropagation(); setActive(Math.max(0, selected)); setOpen(!open); }}>
      <span className="canvas-select-value">{getOptionImage?.(String(value)) && <img className="canvas-select-cover" src={getOptionImage(String(value))} alt="" />}<span>{options[selected]?.label || options[0]?.label}</span></span><ChevronDown size={16} aria-hidden="true" />
    </button>
    {open && position && createPortal(<div ref={popup} id={id} role="listbox" aria-label={label || props["aria-label"] || "选项"} className="canvas-select-popup nodrag nopan nowheel" style={position} onPointerDown={event => event.stopPropagation()} onWheel={event => event.stopPropagation()}>
      {options.map((option, index) => <div key={`${option.value}-${index}`} id={`${id}-${index}`} role="option" aria-selected={index === selected} aria-disabled={option.disabled || undefined} data-option-index={index} className={`canvas-select-option ${index === active ? "active" : ""}`} onPointerMove={() => { if (!option.disabled) setActive(index); }} onMouseDown={event => event.preventDefault()} onClick={() => choose(index)}><span className="canvas-select-value">{getOptionImage?.(option.value) && <img className="canvas-select-cover" src={getOptionImage(option.value)} alt="" loading="lazy" />}<span>{option.label}</span></span>{index === selected && <Check size={16} aria-hidden="true" />}</div>)}
    </div>, trigger.current?.closest("dialog") || document.body)}
  </>;
}
