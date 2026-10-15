import { useEffect, useRef } from "react";

// React Flow forwards node data through its store after rendering. Keep the live
// DOM value owned by the input so delayed node updates cannot reset IME selection.
export default function CanvasTextField({ multiline = false, value = "", onValueChange, className = "", ...props }) {
  const input = useRef(null);
  const composing = useRef(false);
  const initialValue = useRef(value);
  const emittedValue = useRef(value);
  function publish(next) {
    if (next !== emittedValue.current) { emittedValue.current = next; onValueChange(next); }
  }
  useEffect(() => {
    const element = input.current;
    if (element && !composing.current && document.activeElement !== element) {
      if (element.value !== value) element.value = value;
      emittedValue.current = value;
    }
  }, [value]);
  const Tag = multiline ? "textarea" : "input";
  return <Tag {...props} ref={input} defaultValue={initialValue.current} className={`nodrag nopan nowheel ${className}`.trim()}
    onCompositionStart={() => { composing.current = true; }}
    onCompositionEnd={(event) => { composing.current = false; publish(event.currentTarget.value); }}
    onChange={(event) => { if (!composing.current && !event.nativeEvent.isComposing) publish(event.currentTarget.value); }}
    onBlur={(event) => { composing.current = false; publish(event.currentTarget.value); }}
    onKeyDown={(event) => event.stopPropagation()}
    onKeyUp={(event) => event.stopPropagation()}
  />;
}
