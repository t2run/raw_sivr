import { useEffect, useLayoutEffect, useRef, useState } from "react";

/** Zoom state shared by every pane: (u, v) is the point of the image, as a
 *  fraction of its size, that sits in the middle of the pane. */
export interface Zoom {
  on: boolean;
  u: number;
  v: number;
}

/**
 * Shows a photo fitted to its pane, or at 100% (one image pixel per screen
 * pixel) when zoomed. Click to zoom in at that spot, click again to zoom
 * out, drag to pan.
 */
export default function ZoomImage({ src, zoom, onZoom }: { src: string; zoom: Zoom; onZoom: (z: Zoom) => void }) {
  const box = useRef<HTMLDivElement>(null);
  const [nat, setNat] = useState({ w: 0, h: 0 });
  const [size, setSize] = useState({ w: 0, h: 0 });
  const drag = useRef<{ x: number; y: number; u: number; v: number; moved: boolean } | null>(null);

  useEffect(() => setNat({ w: 0, h: 0 }), [src]);

  useLayoutEffect(() => {
    const el = box.current!;
    const measure = () => setSize({ w: el.clientWidth, h: el.clientHeight });
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Size of the image at 100%, in CSS pixels.
  const zw = nat.w / window.devicePixelRatio;
  const zh = nat.h / window.devicePixelRatio;
  const zoomed = zoom.on && nat.w > 0;

  const place = (full: number, view: number, frac: number) =>
    full <= view ? (view - full) / 2 : Math.min(0, Math.max(view - full, view / 2 - frac * full));

  const down = (e: React.PointerEvent) => {
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = { x: e.clientX, y: e.clientY, u: zoom.u, v: zoom.v, moved: false };
  };
  const move = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const dx = e.clientX - d.x;
    const dy = e.clientY - d.y;
    if (Math.abs(dx) + Math.abs(dy) > 4) d.moved = true;
    if (zoomed && d.moved) {
      const clamp = (v: number) => Math.min(1, Math.max(0, v));
      onZoom({ on: true, u: clamp(d.u - dx / zw), v: clamp(d.v - dy / zh) });
    }
  };
  const up = (e: React.PointerEvent) => {
    const d = drag.current;
    drag.current = null;
    if (!d || d.moved || !nat.w) return;
    if (zoomed) {
      onZoom({ ...zoom, on: false });
      return;
    }
    // Zoom in on the spot that was clicked.
    const r = box.current!.getBoundingClientRect();
    const fit = Math.min(r.width / nat.w, r.height / nat.h);
    const u = (e.clientX - r.left - (r.width - nat.w * fit) / 2) / (nat.w * fit);
    const v = (e.clientY - r.top - (r.height - nat.h * fit) / 2) / (nat.h * fit);
    onZoom({ on: true, u: Math.min(1, Math.max(0, u)), v: Math.min(1, Math.max(0, v)) });
  };

  return (
    <div
      ref={box}
      onPointerDown={down}
      onPointerMove={move}
      onPointerUp={up}
      className={`relative h-full w-full overflow-hidden ${zoomed ? "cursor-grab active:cursor-grabbing" : "cursor-zoom-in"}`}
    >
      <img
        src={src}
        draggable={false}
        onLoad={(e) => setNat({ w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })}
        className={zoomed ? "absolute max-w-none" : "h-full w-full object-contain"}
        style={
          zoomed
            ? { width: zw, height: zh, left: place(zw, size.w, zoom.u), top: place(zh, size.h, zoom.v) }
            : undefined
        }
      />
    </div>
  );
}
