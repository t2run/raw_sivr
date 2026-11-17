import { useEffect, useLayoutEffect, useRef, useState } from "react";

/** Zoom state shared by every pane: (u, v) is the point of the image, as a
 *  fraction of its size, that sits in the middle of the pane. scale is the
 *  magnification when on, where 1 means 100% (one image pixel per screen
 *  pixel). */
export interface Zoom {
  on: boolean;
  u: number;
  v: number;
  scale: number;
}

/** Highest magnification, as a multiple of 100%. */
export const MAX_SCALE = 8;
/** Magnification change for one wheel notch or one +/- key press. */
export const ZOOM_STEP = 1.25;

const clamp01 = (v: number) => Math.min(1, Math.max(0, v));

/**
 * Multiplies the magnification by factor. fit is the scale at which the image
 * fills its pane; zooming out to it or below returns to the fitted view, and
 * zooming in from the fitted view starts from it. (u, v) is the new centre.
 */
export function stepZoom(z: Zoom, factor: number, fit: number, u = z.u, v = z.v): Zoom {
  const from = z.on ? z.scale : fit;
  const scale = Math.min(Math.max(MAX_SCALE, fit), from * factor);
  if (factor < 1 && scale <= fit * 1.001) return { ...z, on: false };
  return { on: true, u: clamp01(u), v: clamp01(v), scale };
}

/**
 * Shows a photo fitted to its pane, or magnified when zoomed. Click to zoom
 * to 100% at that spot, click again to go back to fit, scroll the mouse
 * wheel to zoom in and out around the pointer, drag to pan.
 */
export default function ZoomImage({
  src,
  zoom,
  onZoom,
  onFit,
}: {
  src: string;
  zoom: Zoom;
  onZoom: (z: Zoom) => void;
  /** Reports the scale at which the image fits its pane. */
  onFit?: (fit: number) => void;
}) {
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
  const w1 = nat.w / window.devicePixelRatio;
  const h1 = nat.h / window.devicePixelRatio;
  const fit = nat.w > 0 && size.w > 0 ? Math.min(size.w / w1, size.h / h1) : 1;
  const zoomed = zoom.on && nat.w > 0;
  // Size of the image as currently shown.
  const zw = w1 * zoom.scale;
  const zh = h1 * zoom.scale;

  useEffect(() => {
    if (nat.w > 0 && size.w > 0) onFit?.(fit);
  }, [fit, nat.w, size.w, onFit]);

  const place = (full: number, view: number, frac: number) =>
    full <= view ? (view - full) / 2 : Math.min(0, Math.max(view - full, view / 2 - frac * full));

  // The wheel listener is attached natively so it can be non-passive; it
  // reads the latest values through this ref.
  const latest = useRef({ zoom, onZoom, nat, size, fit, zoomed, w1, h1 });
  latest.current = { zoom, onZoom, nat, size, fit, zoomed, w1, h1 };

  useEffect(() => {
    const el = box.current!;
    const wheel = (e: WheelEvent) => {
      const { zoom, onZoom, nat, size, fit, zoomed, w1, h1 } = latest.current;
      if (!nat.w || e.deltaY === 0) return;
      e.preventDefault();
      // Pixel deltas from touchpads are small and many; a mouse notch is ~100.
      const px = e.deltaMode === 1 ? e.deltaY * 33 : e.deltaMode === 2 ? e.deltaY * 100 : e.deltaY;
      const factor = Math.pow(ZOOM_STEP, -Math.max(-3, Math.min(3, px / 100)));
      const r = el.getBoundingClientRect();
      const cx = e.clientX - r.left;
      const cy = e.clientY - r.top;
      // Where the image is now, and which point of it is under the pointer.
      const s = zoomed ? zoom.scale : fit;
      const fw = w1 * s;
      const fh = h1 * s;
      const left = zoomed ? place(fw, size.w, zoom.u) : (size.w - fw) / 2;
      const top = zoomed ? place(fh, size.h, zoom.v) : (size.h - fh) / 2;
      const pu = (cx - left) / fw;
      const pv = (cy - top) / fh;
      // Keep that point under the pointer at the new scale.
      const next = stepZoom({ ...zoom, on: zoomed }, factor, fit);
      if (!next.on) return onZoom(next);
      const nw = w1 * next.scale;
      const nh = h1 * next.scale;
      onZoom({
        ...next,
        u: clamp01((size.w / 2 - (cx - pu * nw)) / nw),
        v: clamp01((size.h / 2 - (cy - pv * nh)) / nh),
      });
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);

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
      onZoom({ ...zoom, on: true, u: clamp01(d.u - dx / zw), v: clamp01(d.v - dy / zh) });
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
    // Zoom to 100% on the spot that was clicked.
    const r = box.current!.getBoundingClientRect();
    const f = Math.min(r.width / nat.w, r.height / nat.h);
    const u = (e.clientX - r.left - (r.width - nat.w * f) / 2) / (nat.w * f);
    const v = (e.clientY - r.top - (r.height - nat.h * f) / 2) / (nat.h * f);
    onZoom({ on: true, u: clamp01(u), v: clamp01(v), scale: 1 });
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
