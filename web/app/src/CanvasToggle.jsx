import { Check } from "lucide-react";

export default function CanvasToggle({ checked, onChange, children, disabled = false, className = "" }) {
  return <button type="button" className={`workflow-toggle nodrag ${className}`.trim()} aria-pressed={checked} disabled={disabled} onClick={() => onChange(!checked)}>
    <span className="workflow-toggle-box" aria-hidden="true">{checked && <Check size={13} strokeWidth={3} />}</span>
    <span>{children}</span>
  </button>;
}
